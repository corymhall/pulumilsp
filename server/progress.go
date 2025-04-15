package server

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"sync"

	"github.com/corymhall/pulumilsp/debug"
	"github.com/corymhall/pulumilsp/lsp"
	"github.com/corymhall/pulumilsp/xcontext"
	"golang.org/x/exp/rand"
)

// A Tracker reports the progress of a long-running operation to an LSP client.
type Tracker struct {
	client                   lsp.Client
	supportsWorkDoneProgress bool

	mu         sync.Mutex
	inProgress map[lsp.ProgressToken]*WorkDone
}

// NewTracker returns a new Tracker that reports progress to the
// specified client.
func NewTracker(client lsp.Client) *Tracker {
	return &Tracker{
		client:     client,
		inProgress: make(map[lsp.ProgressToken]*WorkDone),
	}
}

// SetSupportsWorkDoneProgress sets whether the client supports "work done"
// progress reporting. It must be set before using the tracker.
func (t *Tracker) SetSupportsWorkDoneProgress(b bool) {
	t.supportsWorkDoneProgress = b
}

// WorkDone represents a unit of work that is reported to the client via the
// progress API.
type WorkDone struct {
	client lsp.Client
	// If token is nil, this workDone object uses the ShowMessage API, rather
	// than $/progress.
	token lsp.ProgressToken
	// err is set if progress reporting is broken for some reason (for example,
	// if there was an initial error creating a token).
	err error

	cancelMu  sync.Mutex
	cancelled bool
	cancel    func()

	cleanup func()
}

func (wd *WorkDone) doCancel() {
	wd.cancelMu.Lock()
	defer wd.cancelMu.Unlock()
	if !wd.cancelled {
		wd.cancel()
	}
}

func (t *Tracker) Start(ctx context.Context, title, message string, token lsp.ProgressToken, cancel func()) *WorkDone {
	ctx = xcontext.Detach(ctx)
	wd := &WorkDone{
		client: t.client,
		token:  token,
		cancel: cancel,
	}
	if !t.supportsWorkDoneProgress {
		if err := wd.client.ShowMessage(ctx, &lsp.ShowMessageParams{
			Type:    4, // log
			Message: message,
		}); err != nil {
			debug.LogError(ctx, "error showing message", err)
		}
		return wd
	}

	if wd.token == nil {
		token = strconv.FormatInt(rand.Int63(), 10)
		err := wd.client.WorkDoneProgressCreate(ctx, &lsp.WorkDoneProgressCreateParams{
			Token: token,
		})
		if err != nil {
			debug.LogError(ctx, "error creating progress token", err)
			wd.err = err
			return wd
		}
		wd.token = token
	}
	t.mu.Lock()
	t.inProgress[wd.token] = wd
	t.mu.Unlock()
	wd.cleanup = func() {
		t.mu.Lock()
		delete(t.inProgress, token)
		t.mu.Unlock()
	}
	err := wd.client.ProgressBegin(ctx, &lsp.WorkDoneProgressBeginParams{
		Token: wd.token,
		Value: &lsp.WorkDoneProgressBeginValue{
			Kind:        "begin",
			Title:       title,
			Cancellable: wd.cancel != nil,
			Message:     message,
		},
	})
	if err != nil {
		debug.LogError(ctx, "error starting progress", err)
	}
	return wd
}

// End reports a workdone completion back to the client.
func (wd *WorkDone) End(ctx context.Context, message string) {
	ctx = xcontext.Detach(ctx) // progress messages should not be cancelled
	if wd == nil {
		debug.LogError(ctx, "work done error", errors.New("end called on nil work done"))
		return
	}
	var err error
	switch {
	case wd.err != nil:
		// There is a prior error.
	case wd.token == nil:
		// We're falling back to message-based reporting.
		err = wd.client.ShowMessage(ctx, &lsp.ShowMessageParams{
			Type:    3, // Info
			Message: message,
		})
	default:
		err = wd.client.ProgressEnd(ctx, &lsp.WorkDoneProgressEndParams{
			Token: wd.token,
			Value: &lsp.WorkDoneProgressEndValue{
				Kind:    "end",
				Message: message,
			},
		})
	}
	if err != nil {
		debug.LogError(ctx, "error ending progress", err)
	}
	if wd.cleanup != nil {
		wd.cleanup()
	}
}

func (t *Tracker) Cancel(token lsp.ProgressToken) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	wd, ok := t.inProgress[token]
	if !ok {
		return fmt.Errorf("token %q not found in progress", token)
	}
	if wd.cancel == nil {
		return fmt.Errorf("work %q is not cancellable", token)
	}
	wd.doCancel()
	return nil
}
