package server

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/corymhall/pulumilsp/ai"
	"github.com/corymhall/pulumilsp/file"
	"github.com/corymhall/pulumilsp/lsp"
	"github.com/corymhall/pulumilsp/parser"
	"github.com/corymhall/pulumilsp/xcontext"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	tree_sitter "github.com/tree-sitter/go-tree-sitter"
	tree_sitter_typescript "github.com/tree-sitter/tree-sitter-typescript/bindings/go"
)

var viewIndex int64

// New creates an LSP server and binds it to handle incoming client
// messages on the supplied stream.
func New(logger *log.Logger, client lsp.Client) lsp.Server {
	const concurrentAnalyses = 1
	napper, err := parser.NewResourceNapper(tree_sitter.NewLanguage(tree_sitter_typescript.LanguageTypescript()))
	contract.AssertNoErrorf(err, "failed to create resource napper: %v", err)
	// If this assignment fails to compile after a protocol
	// upgrade, it means that one or more new methods need new
	// stub declarations in unimplemented.go.
	return &server{
		logger:          logger,
		client:          client,
		napper:          napper,
		diagnostics:     make(map[lsp.DocumentURI]*fileDiagnostics),
		diagnosticsSema: make(chan unit, concurrentAnalyses),
		progress:        NewTracker(client, logger),
		aiClient:        ai.NewClient(logger),
	}
}
func (s *server) GetCapturesFromURI(ctx context.Context, uri lsp.DocumentURI) ([]parser.CaptureInfo, error) {
	snapshot, release, err := s.view.Snapshot()
	defer release()
	if err != nil {
		return nil, err
	}
	handle, err := snapshot.ReadFile(ctx, uri)
	if err != nil {
		return nil, err
	}
	contents, err := handle.Content()
	if err != nil {
		return nil, err
	}
	return s.napper.GetCapturesFromFile(contents)
}

type serverState int

const (
	serverCreated      = serverState(iota)
	serverInitializing // set once the server has received "initialize" request
	serverInitialized  // set once the server has received "initialized" request
	serverShutDown
)

func (s serverState) String() string {
	switch s {
	case serverCreated:
		return "created"
	case serverInitializing:
		return "initializing"
	case serverInitialized:
		return "initialized"
	case serverShutDown:
		return "shutDown"
	}
	return fmt.Sprintf("(unknown state: %d)", int(s))
}

type unit struct{}

type server struct {
	logger   *log.Logger
	client   lsp.Client
	stateMu  sync.Mutex
	aiClient *ai.Client
	state    serverState
	// Map of file names to their contents
	rootURI lsp.DocumentURI

	// view is the view associated with this server
	view *View

	// snapshots is a counting semaphore that records the number
	// of unreleased snapshots associated with this session.
	// Shutdown waits for it to fall to zero.
	snapshotWG sync.WaitGroup

	// napper is the parser used to extract resource information from
	// files.
	napper *parser.ResourceNapper

	// progress is the progress tracker used to report progress
	// to the client.
	progress *Tracker

	diagnosticsMu sync.Mutex // guards map and its values
	diagnostics   map[lsp.DocumentURI]*fileDiagnostics
	// diagnosticsSema limits the concurrency of diagnostics runs, which can be
	// expensive.
	diagnosticsSema chan unit

	criticalErrorStatusMu sync.Mutex
	criticalErrorStatus   *WorkDone

	modificationMu        sync.Mutex
	cancelPrevDiagnostics func()
	lastModificationID    uint64 // incrementing clock
}

func (s *server) Logger() *log.Logger {
	return s.logger
}

// Initialize the view for the server.
// This only needs to be done once, since there is only one view per session
// TODO: add stuff here that needs to happen during initialization
func (s *server) initializeView(ctx context.Context) {
	if s.view == nil {
		_, snapshot, release, err := s.NewView(ctx, s.rootURI)
		if err != nil {
			s.logger.Printf("error creating view: %v", err)
		}

		var nsnapshots sync.WaitGroup
		initialized := make(chan struct{})
		nsnapshots.Add(1)
		work := s.progress.Start(ctx, "Pulumi", "Calculating initial diagnostics...", nil, nil)
		go func() {
			snapshot.AwaitInitialized(ctx)
			nsnapshots.Done()
			close(initialized)
		}()

		go func() {
			<-initialized
			s.diagnoseSnapshot(ctx, snapshot, nil, 0)
			release()
			work.End(ctx, "Done.")
		}()

		// wait for snapshots to be initialized, but don't wait for diagnosis to finish
		nsnapshots.Wait()
	}
}

func (s *server) NewView(ctx context.Context, root lsp.DocumentURI) (*View, *Snapshot, func(), error) {
	contract.Assertf(s.view == nil, "NewView called when view already exists")

	dir := root.Path()
	pulumiyaml := filepath.Join(dir, "Pulumi.yaml")
	def := &viewDefinition{
		root:       root,
		pulumiyaml: lsp.URIFromPath(pulumiyaml),
	}

	view, snapshot, release := s.createView(ctx, def)
	s.view = view
	return view, snapshot, release, nil
}

func (s *server) createView(ctx context.Context, def *viewDefinition) (*View, *Snapshot, func()) {
	contract.Assertf(s.view == nil, "createView called when view already exists")

	index := atomic.AddInt64(&viewIndex, 1)
	// create a background context for the view
	baseCtx := xcontext.Detach(ctx)
	backgroundCtx, cancel := context.WithCancel(baseCtx)
	v := &View{
		id:                   strconv.FormatInt(index, 10),
		baseCtx:              baseCtx,
		initialWorkspaceLoad: make(chan struct{}),
		initializationSema:   make(chan struct{}, 1),
		viewDefinition:       def,
	}
	s.snapshotWG.Add(1)
	v.snapshot = &Snapshot{
		view:              v,
		backgroundCtx:     backgroundCtx,
		cancel:            cancel,
		policyDiagnostics: make(map[lsp.DocumentURI]any),
		files:             make(fileMap),
		refcount:          1,
		done:              s.snapshotWG.Done,
	}

	initCtx, cancel := context.WithCancel(xcontext.Detach(ctx))
	v.cancelInitialWorkspaceLoad = cancel

	snapshot := v.snapshot
	bgRelease := snapshot.Acquire()
	go func() {
		defer bgRelease()
		snapshot.initialize(initCtx, true)
	}()
	return v, snapshot, snapshot.Acquire()
}

func (s *server) updateCriticalErrorStatus(ctx context.Context, snapshot *Snapshot, err *InitializationError) {
	s.criticalErrorStatusMu.Lock()
	defer s.criticalErrorStatusMu.Unlock()

	var errMsg string
	if err != nil {
		errMsg = strings.ReplaceAll(err.MainError.Error(), "\n", " ")
	}

	if s.criticalErrorStatus == nil {
		if errMsg != "" {
			s.criticalErrorStatus = s.progress.Start(ctx, "Error loading workspace", errMsg, nil, nil)
		}
		return
	}

	// if an error is already present, update it or mark it as resolved
	if errMsg == "" {
		s.criticalErrorStatus.End(ctx, "Done.")
		s.criticalErrorStatus = nil
	}
}

func (s *server) invalidateViewLocked(ctx context.Context, changed StateChange) (*Snapshot, func()) {
	ctx = xcontext.Detach(ctx)
	view := s.view
	view.snapshotMu.Lock()
	defer view.snapshotMu.Unlock()
	prevSnapshot := view.snapshot
	if prevSnapshot == nil {
		panic("invalidateContent called after shutdown")
	}

	isSave := slices.ContainsFunc(changed.Modifications, func(mod file.Modification) bool {
		return mod.Action == file.Save
	})
	if isSave {
		// only cancel all still-running previous requests if this is a save, since they would be
		// operating on stale data
		prevSnapshot.cancel()
	}

	prevSnapshot.AwaitInitialized(ctx)

	s.snapshotWG.Add(1)
	view.snapshot = prevSnapshot.clone(view.baseCtx, changed, s.snapshotWG.Done)
	prevSnapshot.decref()
	return view.snapshot, view.snapshot.Acquire()
}

// Shutdown implements the 'shutdown' LSP handler. It releases resources
// associated with the server and waits for all ongoing work to complete.
func (s *server) Shutdown(ctx context.Context) error {
	s.stateMu.Lock()
	defer s.stateMu.Unlock()
	if s.state != serverShutDown {
		// drop all the active views
		s.state = serverShutDown
		s.view.shutdown()
		s.view = nil
		s.snapshotWG.Wait() // wait for all work on associated snapshots to finish
		s.napper.Close()
	}
	return nil
}

func (s *server) Exit(ctx context.Context) error {
	s.stateMu.Lock()
	defer s.stateMu.Unlock()
	if s.state != serverShutDown {
		os.Exit(1)
	}
	return nil
}
