package server

import (
	"context"
	"errors"
	"sync"

	"github.com/corymhall/pulumilsp/file"
	"github.com/corymhall/pulumilsp/lsp"
)

type StateChange struct {
	Modifications     []file.Modification
	Files             fileMap
	PolicyDiagnostics map[lsp.DocumentURI]any // TODO:
}

type View struct {
	id string // a unique string to identify this view in (e.g.) serialized Commands

	*viewDefinition

	// baseCtx is the context handed to NewView. This is the parent of all
	// background contexts created for this view.
	baseCtx context.Context

	snapshotMu sync.Mutex
	snapshot   *Snapshot // latest snapshot; nil after shutdown has been called

	// initializationSema is used limit concurrent initialization of snapshots in
	// the view. We use a channel instead of a mutex to avoid blocking when a
	// context is canceled.
	//
	// This field (along with snapshot.initialized) guards against duplicate
	// initialization of snapshots. Do not change it without adjusting snapshot
	// accordingly.
	initializationSema chan struct{}

	initialWorkspaceLoad       chan struct{}
	cancelInitialWorkspaceLoad func() // cancel the initial workspace load
}

// shutdown releases resources associated with the view.
func (v *View) shutdown() {
	// cancel the initial workspace load if it is still running
	v.cancelInitialWorkspaceLoad()
	v.snapshotMu.Lock()
	if v.snapshot != nil {
		v.snapshot.cancel()
		v.snapshot.decref()
		v.snapshot = nil
	}
	v.snapshotMu.Unlock()
}

type viewDefinition struct {
	root       lsp.DocumentURI // root directory; where to run the Pulumi command
	pulumiyaml lsp.DocumentURI // the nearest Pulumi.yaml file
}

// Snapshot returns the current snapshot for the view, and a
// release function that must be called when the Snapshot is
// no longer needed.
//
// The resulting error is non-nil if and only if the view is shut down, in
// which case the resulting release function will also be nil.
func (v *View) Snapshot() (*Snapshot, func(), error) {
	v.snapshotMu.Lock()
	defer v.snapshotMu.Unlock()
	if v.snapshot == nil {
		return nil, nil, errors.New("view is shutdown")
	}
	return v.snapshot, v.snapshot.Acquire(), nil
}
