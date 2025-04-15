package server

import (
	"context"

	"github.com/corymhall/pulumilsp/debug"
	"github.com/corymhall/pulumilsp/file"
	"github.com/corymhall/pulumilsp/lsp"
	"github.com/corymhall/pulumilsp/xcontext"
)

// ModificationSource identifies the origin of a change.
type ModificationSource int

const (
	// FromDidOpen is from a didOpen notification.
	FromDidOpen = ModificationSource(iota)

	// FromDidChange is from a didChange notification.
	FromDidChange

	// FromDidChangeWatchedFiles is from didChangeWatchedFiles notification.
	FromDidChangeWatchedFiles

	// FromDidSave is from a didSave notification.
	FromDidSave

	// FromDidClose is from a didClose notification.
	FromDidClose

	// FromDidChangeConfiguration is from a didChangeConfiguration notification.
	FromDidChangeConfiguration

	// FromRegenerateCgo refers to file modifications caused by regenerating
	// the cgo sources for the workspace.
	FromRegenerateCgo

	// FromInitialWorkspaceLoad refers to the loading of all packages in the
	// workspace when the view is first created.
	FromInitialWorkspaceLoad

	// FromCheckUpgrades refers to state changes resulting from the CheckUpgrades
	// command, which queries module upgrades.
	FromCheckUpgrades

	// FromResetGoModDiagnostics refers to state changes resulting from the
	// ResetGoModDiagnostics command.
	FromResetGoModDiagnostics

	// FromToggleCompilerOptDetails refers to state changes resulting from toggling
	// a package's compiler optimization details flag.
	FromToggleCompilerOptDetails
)

func (s *server) didModifyFiles(ctx context.Context, modifications []file.Modification, cause ModificationSource) error {
	ctx, done := debug.Start(ctx, "textdocument.didModifyFiles")
	defer done()
	changes := []lsp.DocumentURI{}
	for _, mod := range modifications {
		changes = append(changes, mod.URI)
		s.mustPublishDiagnostics(mod.URI)
	}

	changed := make(fileMap)
	for _, m := range modifications {
		fh := mustReadFile(ctx, m.URI)
		changed[m.URI] = fh
	}

	snapshot, release := s.invalidateViewLocked(ctx, StateChange{Modifications: modifications, Files: changed})
	release()
	ctx, _ = debug.With(ctx, "snapshotSequenceID", snapshot.sequenceID)

	modCtx, modID := s.updateViewsToDiagnose(ctx)
	// don't block on diagnostics
	go func() {
		s.diagnoseChangedView(modCtx, modID, changes, cause)
	}()

	return nil
}

func (s *server) updateViewsToDiagnose(ctx context.Context) (context.Context, uint64) {
	s.modificationMu.Lock()
	defer s.modificationMu.Unlock()
	if s.cancelPrevDiagnostics != nil {
		s.cancelPrevDiagnostics()
	}
	modCtx := xcontext.Detach(ctx)
	modCtx, s.cancelPrevDiagnostics = context.WithCancel(modCtx)
	s.lastModificationID++
	modID := s.lastModificationID
	modCtx, _ = debug.With(modCtx, "modificationID", modID)
	return modCtx, modID
}
