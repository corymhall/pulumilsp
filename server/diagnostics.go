package server

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/corymhall/pulumilsp/lsp"
	"github.com/corymhall/pulumilsp/parser"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	rpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
)

type (
	diagMap = map[lsp.DocumentURI][]*Diagnostic
)

// fileDiagnostics holds the current state of published diagnostics for a file.
type fileDiagnostics struct {
	mustPublish    bool // if set, publish diagnostics even if they haven't changed
	viewDiagnostic *viewDiagnostics
}

// viewDiagnostics holds a set of file diagnostics computed from a given View.
type viewDiagnostics struct {
	snapshot    uint64 // snapshot sequence ID
	version     int32  // file version
	diagnostics []*Diagnostic
}

func (s *server) mustPublishDiagnostics(uri lsp.DocumentURI) {
	s.diagnosticsMu.Lock()
	defer s.diagnosticsMu.Unlock()

	if s.diagnostics[uri] == nil {
		s.diagnostics[uri] = new(fileDiagnostics)
	}
	s.diagnostics[uri].mustPublish = true
}

func (s *server) diagnoseSnapshot(ctx context.Context, snapshot *Snapshot, changedURIs []lsp.DocumentURI, delay time.Duration) {
	s.logger.Printf("diagnoseSnapshot: %d", snapshot.SequenceID())
	diagnostics, err := s.diagnose(ctx, snapshot)
	if err != nil {
		s.logger.Printf("error diagnosing snapshot: %v", err)
		return
	}

	s.updateDiagnostics(ctx, snapshot, diagnostics)
}

func (s *server) diagnoseChangedView(ctx context.Context, modID uint64, lastChange []lsp.DocumentURI, cause ModificationSource) {
	s.logger.Printf("diagnoseChangedView: %d views to diagnose", modID)

	snapshot, release, err := s.view.Snapshot()
	if err != nil {
		s.logger.Printf("error getting snapshot: %v", err)
		return
	}
	defer release()

	if cause != FromDidSave {
		s.logger.Printf("diagnoseChangedView: cause %v", cause)
		// only update on save
		return
	}

	work := s.progress.Start(ctx, "Pulumi", "Running preview...", nil, nil)
	s.diagnoseSnapshot(ctx, snapshot, lastChange, 0 /* delay */)
	work.End(ctx, "Done.")
}

func (s *server) publishFileDiagnostics(ctx context.Context, uri lsp.DocumentURI, f *fileDiagnostics) error {
	if err := s.client.PublishDiagnostics(ctx, &lsp.PublishDiagnosticsParams{
		Diagnostics: toProtocolDiagnostics(f.viewDiagnostic.diagnostics),
		URI:         uri,
		Version:     f.viewDiagnostic.version,
	}); err != nil {
		s.logger.Printf("error publishing diagnostics: %v", err)
		return err
	}
	return nil
}

func toProtocolDiagnostics(diags []*Diagnostic) []lsp.Diagnostic {
	reports := []lsp.Diagnostic{}
	for _, diag := range diags {
		pdiag := lsp.Diagnostic{
			Message:  strings.TrimSpace(diag.Message),
			Range:    diag.Range,
			Severity: diag.Severity,
			Source:   string(diag.Source),
			Data:     diag.Data,
		}
		reports = append(reports, pdiag)
	}
	return reports
}

func (s *server) updateDiagnostics(ctx context.Context, snapshot *Snapshot, diagnostics diagMap) {
	s.diagnosticsMu.Lock()
	defer s.diagnosticsMu.Unlock()

	// before updating diagnostics, check if the context (i.e. snapshot background context)
	// is not  cancelled. That would mean we started diagnosing the next snapshot
	if ctx.Err() != nil {
		s.logger.Printf("context error while updating diagnostics for snapshot %d: %v", snapshot.SequenceID(), ctx.Err())
		return
	}

	// updateAndPublish updates diagnostics for a file.
	// Because we only update diagnostics on save, we always overwrite existing
	// diagnostics.
	updateAndPublish := func(uri lsp.DocumentURI, f *fileDiagnostics, diags []*Diagnostic) error {
		fh, err := snapshot.ReadFile(ctx, uri)
		if err != nil {
			return err
		}
		f.viewDiagnostic = &viewDiagnostics{
			snapshot:    snapshot.SequenceID(),
			version:     fh.Version(),
			diagnostics: diags,
		}

		return s.publishFileDiagnostics(ctx, uri, f)
	}

	seen := make(map[lsp.DocumentURI]bool)
	for uri, diags := range diagnostics {
		f, ok := s.diagnostics[uri]
		if !ok {
			f = &fileDiagnostics{}
			s.diagnostics[uri] = f
		}
		seen[uri] = true
		if err := updateAndPublish(uri, f, diags); err != nil {
			s.logger.Printf("context error while updating diagnostics: %v", ctx.Err())
			if ctx.Err() != nil {
				return
			}
		}
	}

	// clean up files that have no diagnostics
	for uri, f := range s.diagnostics {
		if !seen[uri] {
			if err := updateAndPublish(uri, f, nil); err != nil {
				s.logger.Printf("context error while updating diagnostics: %v", ctx.Err())
				if ctx.Err() != nil {
					return
				}
			}
		}
	}
}

func (s *server) diagnose(ctx context.Context, snapshot *Snapshot) (diagMap, error) {
	// wait for a free diagnostics slot
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case s.diagnosticsSema <- struct{}{}:
	}

	// defer release the semaphore
	defer func() {
		<-s.diagnosticsSema
	}()

	diagnostics := make(diagMap)

	initialErr := snapshot.InitializationError()
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	s.updateCriticalErrorStatus(ctx, snapshot, initialErr)
	runner := snapshot.PulumiCmdRunner()
	if runner == nil {
		return nil, fmt.Errorf("no runner")
	}

	resources, err := runner.Run(ctx, s.logger)
	// TODO: we need to differentiate between critical errors
	// (i.e. errors that prevent any results) and errors on individual resources
	if err != nil {
		s.logger.Printf("error running Run: %v", err)
		s.updateCriticalErrorStatus(ctx, snapshot, &InitializationError{
			MainError: err,
		})
		return nil, err
	}
	s.logger.Printf("diagnostics for %d resources", len(resources))

	fileCaptures := make(map[lsp.DocumentURI][]parser.CaptureInfo)
	for urn, info := range resources {
		if info.Diagnostics == nil || info.SourcePosition == nil {
			s.logger.Printf("diagnostics for %s: no diagnostics or source position", urn)
			continue
		}
		uri := lsp.DocumentURI(info.SourcePosition.Uri)
		if _, ok := fileCaptures[uri]; !ok {
			captures, err := s.GetCapturesFromURI(ctx, uri)
			if err != nil {
				s.logger.Printf("error getting captures from URI %s: %v", uri, err)
				continue
			}
			fileCaptures[uri] = captures
		}
		infos := fileCaptures[uri]
		s.logger.Printf("diagnostics for %s", uri)
		diags := []*Diagnostic{}
		for _, diag := range info.Diagnostics {
			s.logger.Printf("diagnostics for %s: %s at %d %d", uri, diag.Message, info.SourcePosition.Line, info.SourcePosition.Column)
			diagCapture := findCaptureWithStartLine(infos, info.SourcePosition.Line-1)
			data := lsp.CodeActionResolveData{
				CaptureInfo: *diagCapture,
				URI:         uri,
			}
			rawData, err := json.Marshal(data)
			if err != nil {
				s.logger.Printf("error marshalling capture: %v", err)
				continue
			}
			msg := json.RawMessage(rawData)
			diags = append(diags, &Diagnostic{
				Range: lsp.Range{
					Start: lsp.Position{
						Line:      int32(diagCapture.StartPoint.Row),
						Character: int32(diagCapture.StartPoint.Column),
					},
					End: lsp.Position{
						Line:      int32(diagCapture.EndPoint.Row),
						Character: int32(diagCapture.EndPoint.Column),
					},
				},
				Data:     &msg,
				URI:      uri,
				Message:  diag.Message,
				Severity: enforcementLevelToSeverity(diag.EnforcementLevel),
				Source:   DiagnosticSource(diag.PolicyName),
			})
		}
		if len(diags) > 0 {
			if d, ok := diagnostics[uri]; ok {
				// merge the diagnostics
				diags = append(d, diags...)
				diagnostics[uri] = diags
			} else {
				diagnostics[uri] = diags
			}
			diagnostics[uri] = diags
		}
	}

	return diagnostics, nil
}

func findCaptureWithStartLine(captures []parser.CaptureInfo, line int32) *parser.CaptureInfo {
	for _, capture := range captures {
		if capture.StartPoint.Row == uint(line) {
			return &capture
		}
	}
	return nil
}

func enforcementLevelToSeverity(level rpc.EnforcementLevel) lsp.DiagnosticSeverity {
	switch level {
	case rpc.EnforcementLevel_ADVISORY:
		return 3 // information
	case rpc.EnforcementLevel_MANDATORY:
		return 1 // error
	case rpc.EnforcementLevel_REMEDIATE:
		return 2 // warning
	case rpc.EnforcementLevel_DISABLED:
		return 4 // hint
	default:
		contract.Failf("unknown enforcement level: %v", level)
	}
	return 0
}
