package server

import (
	"context"
	"fmt"
	"maps"
	"sync"

	"github.com/corymhall/pulumilsp/file"
	"github.com/corymhall/pulumilsp/lsp"
	"github.com/corymhall/pulumilsp/pulumicommand"
	"github.com/pulumi/pulumi/sdk/v3/go/auto"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

type InitializationError struct {
	MainError   error
	Diagnostics map[lsp.DocumentURI][]*Diagnostic
}

type Snapshot struct {
	refMu sync.Mutex

	// files maps file URIs to their corresponding FileHandles.
	// It may invalidated when a file's content changes.
	files fileMap

	sequenceID uint64

	// backgroundCtx is the context used for background snapshot tasks.
	backgroundCtx context.Context
	cancel        func()

	// The view this snapshot is associated with
	view *View

	// initialized reports whether the snapshot has been initialized. Concurrent
	// initialization is guarded by the view.initializationSema. Each snapshot is
	// initialized at most once: concurrent initialization is guarded by
	// view.initializationSema.
	initialized bool

	initialErr *InitializationError

	// limits command concurrency
	pulumicmdRunner *pulumicommand.Runner

	// refcount holds the number of outstanding references to the current
	// Snapshot. When refcount is decremented to 0, the Snapshot maps are
	// destroyed and the done function is called.
	//
	refcount          int
	done              func() // for implementing Session.Shutdown
	policyDiagnostics map[lsp.DocumentURI]any

	// mu guards all of the maps in the snapshot, as well as the builtin URI and
	// initialized.
	mu sync.Mutex
}

func (s *Snapshot) PulumiCmdRunner() *pulumicommand.Runner {
	return s.pulumicmdRunner
}

func (s *Snapshot) InitializationError() *InitializationError {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.initialErr
}

func (s *Snapshot) initialize(ctx context.Context, firstAttempt bool) {
	select {
	case <-ctx.Done():
		return
	case s.view.initializationSema <- struct{}{}:
	}

	defer func() {
		<-s.view.initializationSema
	}()

	s.mu.Lock()
	initialized := s.initialized
	s.mu.Unlock()
	if initialized {
		return
	}

	defer func() {
		if firstAttempt {
			close(s.view.initialWorkspaceLoad)
		}
	}()

	var initialErr *InitializationError
	if s.pulumicmdRunner == nil {
		if err := s.initializeWorkspace(ctx); err != nil {
			initialErr = &InitializationError{
				MainError: err,
			}
		}
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.initialized = true
	s.initialErr = initialErr
}

func (s *Snapshot) AwaitInitialized(ctx context.Context) {
	select {
	case <-ctx.Done():
		return
	case <-s.view.initialWorkspaceLoad:
	}

	s.initialize(ctx, false)
}

func (s *Snapshot) initializeWorkspace(ctx context.Context) error {
	rootPath := s.view.root.Path()
	workspace, err := auto.NewLocalWorkspace(ctx, auto.WorkDir(rootPath))
	if err != nil {
		return fmt.Errorf("error creating workspace: %w", err)
	}

	currentStack, err := workspace.Stack(ctx)
	if err != nil {
		return fmt.Errorf("error getting current stack: %w", err)
	}
	if currentStack == nil {
		return fmt.Errorf("error getting current stack: %w", err)
	}

	stack, err := auto.SelectStack(ctx, currentStack.Name, workspace)
	if err != nil {
		return fmt.Errorf("error selecting stack: %w", err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.pulumicmdRunner = pulumicommand.New(stack)
	return nil
}

func (s *Snapshot) SequenceID() uint64 {
	return s.sequenceID
}

// Acquire prevents the snapshot from being destroyed until the returned
// function is called.
func (s *Snapshot) Acquire() func() {
	s.refMu.Lock()
	defer s.refMu.Unlock()
	contract.Assertf(s.refcount > 0, "non-positive refs")
	s.refcount++

	return s.decref
}

// decref should only be referenced by Acquire, and by View when it frees its
// reference to View.snapshot.
func (s *Snapshot) decref() {
	s.refMu.Lock()
	defer s.refMu.Unlock()

	contract.Assertf(s.refcount > 0, "non-positive refs")
	s.refcount--
	if s.refcount == 0 {
		s.done()
	}
}

// A fileMap maps files in the snapshot, with some additional bookkeeping:
// It keeps track of overlays as well as directories containing any observed
// file.
type fileMap map[lsp.DocumentURI]file.Handle

func (m fileMap) clone(changes fileMap) fileMap {
	newFiles := maps.Clone(m)
	m2 := newFiles

	for uri, fh := range changes {
		if !fileExists(fh) {
			delete(m2, uri)
		}
	}
	for uri, fh := range changes {
		if fileExists(fh) {
			m2[uri] = fh
		}
	}
	return m2
}

// fileExists reports whether the file has a Content (which may be empty).
// An overlay exists even if it is not reflected in the file system.
func fileExists(fh file.Handle) bool {
	_, err := fh.Content()
	return err == nil
}

func (s *Snapshot) ReadFile(ctx context.Context, uri lsp.DocumentURI) (file.Handle, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return lockedSnapshot{s}.ReadFile(ctx, uri)
}

type lockedSnapshot struct {
	s *Snapshot
}

func (s lockedSnapshot) ReadFile(ctx context.Context, uri lsp.DocumentURI) (file.Handle, error) {
	fh, ok := s.s.files[uri]
	if !ok {
		var err error
		fh, err = ReadFile(ctx, uri)
		if err != nil {
			return nil, err
		}
		s.s.files[uri] = fh
	}
	return fh, nil
}

func (s *Snapshot) clone(bgCtx context.Context, changed StateChange, done func()) *Snapshot {
	changedFiles := changed.Files
	s.mu.Lock()
	defer s.mu.Unlock()

	bgCtx, cancel := context.WithCancel(bgCtx)
	diags := maps.Clone(s.policyDiagnostics)
	maps.Copy(diags, changed.PolicyDiagnostics)
	result := &Snapshot{
		sequenceID:        s.sequenceID + 1,
		cancel:            cancel,
		refcount:          1,
		backgroundCtx:     bgCtx,
		pulumicmdRunner:   s.pulumicmdRunner,
		done:              done,
		files:             s.files.clone(changedFiles),
		view:              s.view,
		initialized:       s.initialized,
		policyDiagnostics: diags,
	}

	return result
}
