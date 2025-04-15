package pulumicommand

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"

	"github.com/corymhall/pulumilsp/debug"
	"github.com/nxadm/tail"
	"github.com/pulumi/providertest/grpclog"
	"github.com/pulumi/pulumi/sdk/v3/go/auto"
	"github.com/pulumi/pulumi/sdk/v3/go/auto/optpreview"
	rpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
	jsonpb "google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/reflect/protoreflect"
)

/**
* TODOS:
* TODO: Move the resourcestore into its own package and make resources private
* TODO: add handling errors that occur during preview. e.g. Failures during check, etc
*       should be processed as diagnostics
 */

type ResourceStore struct {
	mutex     sync.Mutex
	Resources map[string]*ResourceInfo
}
type ResourceInfo struct {
	SourcePosition *rpc.SourcePosition
	Diagnostics    []*rpc.AnalyzeDiagnostic
}

func (r *ResourceInfo) SetSourcePosition(pos *rpc.SourcePosition) {
	r.SourcePosition = pos
}

func (r *ResourceInfo) AddDiagnostic(diagnostic *rpc.AnalyzeDiagnostic) {
	r.Diagnostics = append(r.Diagnostics, diagnostic)
}

func (r *ResourceInfo) SetDiagnostics(diagnostics []*rpc.AnalyzeDiagnostic) {
	r.Diagnostics = diagnostics
}

func (r *ResourceStore) GetResourceInfo(urn string) (*ResourceInfo, bool) {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	info, ok := r.Resources[urn]
	return info, ok
}

func (r *ResourceStore) getOrCreateResourceInfo(urn string) *ResourceInfo {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	if r.Resources == nil {
		r.Resources = map[string]*ResourceInfo{}
	}
	if _, ok := r.Resources[urn]; !ok {
		r.Resources[urn] = &ResourceInfo{}
	}
	return r.Resources[urn]
}

func run(ctx context.Context, stack auto.Stack) (map[string]*ResourceInfo, error) {
	store := &ResourceStore{}
	events := make(chan GrpcEntry)

	f, err := setupLogTailing("preview", events)
	if err != nil {
		return nil, fmt.Errorf("failed to tail logs: %w", err)
	}
	defer f.Close()
	ctx, _ = debug.Start(ctx, "pulumi.preview", "filename", f.Filename)
	go processGrpcEvents(ctx, events, store)

	stack.Workspace().SetEnvVar("PULUMI_DEBUG_GRPC", f.Filename)

	_, err = stack.Preview(ctx, optpreview.SuppressProgress())
	return store.Resources, err
}

func processGrpcEvents(ctx context.Context, events <-chan GrpcEntry, store *ResourceStore) {
	for {
		select {
		case <-ctx.Done():
			debug.Debug.Log(ctx, "Stopping processGrpcEvents due to context cancellation")
			return
		case evt, ok := <-events:
			if !ok {
				debug.Debug.Log(ctx, "Events channel closed, stopping processGrpcEvents")
				return
			}
			handleGrpcEvent(ctx, evt, store)
		}
	}
}

func handleGrpcEvent(ctx context.Context, evt GrpcEntry, store *ResourceStore) {
	switch evt.Method {
	case "/pulumirpc.ResourceMonitor/RegisterResource":
		debug.Debug.Log(ctx, "RegisterResource event")
		handleRegisterResource(ctx, evt, store)
	case "/pulumirpc.Analyzer/AnalyzeStack":
		debug.Debug.Log(ctx, "AnalyzeStack event")
		handleAnalyzeStack(ctx, evt, store)
	case "/pulumirpc.Analyzer/Analyze":
		debug.Debug.Log(ctx, "AnalyzeStack event")
		handleAnalyze(ctx, evt, store)
	default:
		// Unhandled method
	}
}

func handleRegisterResource(ctx context.Context, evt GrpcEntry, store *ResourceStore) {
	tEntry, err := unmarshalTypedEntry[rpc.RegisterResourceRequest, rpc.RegisterResourceResponse](evt.GrpcLogEntry)
	if err != nil {
		debug.LogError(ctx, "Error unmarshalling register resource entry", err)
		return
	}
	store.getOrCreateResourceInfo(tEntry.Response.Urn).SetSourcePosition(tEntry.Request.SourcePosition)
}

func handleAnalyzeStack(ctx context.Context, evt GrpcEntry, store *ResourceStore) {
	tEntry, err := unmarshalTypedEntry[rpc.AnalyzeStackRequest, rpc.AnalyzeResponse](evt.GrpcLogEntry)
	if err != nil {
		debug.LogError(ctx, "Error unmarshalling analyze stack entry", err)
		return
	}
	if tEntry.Response.Diagnostics == nil {
		debug.Debug.Log(ctx, "No diagnostics found in analyze stack response for Stack")
		return
	}
	for _, d := range tEntry.Response.Diagnostics {
		store.getOrCreateResourceInfo(d.Urn).AddDiagnostic(d)
	}
}

func handleAnalyze(ctx context.Context, evt GrpcEntry, store *ResourceStore) {
	tEntry, err := unmarshalTypedEntry[rpc.AnalyzeRequest, rpc.AnalyzeResponse](evt.GrpcLogEntry)
	if err != nil {
		debug.LogError(ctx, "Error unmarshalling analyze entry: %v", err)
		return
	}
	if tEntry.Response.Diagnostics == nil {
		debug.Debug.Log(ctx, "No diagnostics found in analyze response for URN", "URN", tEntry.Request.Urn)
		return
	}
	store.getOrCreateResourceInfo(tEntry.Request.Urn).SetDiagnostics(tEntry.Response.Diagnostics)
}

func setupLogTailing(command string, events chan<- GrpcEntry) (*fileWatcher, error) {
	f, err := tailLogs(command, []chan<- GrpcEntry{events})
	if err != nil {
		return nil, err
	}
	return f, nil
}

type optionFunc func(*optpreview.Options)

func PolicyPacks(packs []string) optpreview.Option {
	return optionFunc(func(opts *optpreview.Options) {
		opts.PolicyPacks = packs
	})
}

func (o optionFunc) ApplyOption(opts *optpreview.Options) {
	o(opts)
}

type GrpcEntry struct {
	grpclog.GrpcLogEntry
	Error error
}

type fileWatcher struct {
	Filename  string
	tail      *tail.Tail
	receivers []chan<- GrpcEntry
	done      chan bool
}

func watchFile(path string, receivers []chan<- GrpcEntry) (*fileWatcher, error) {
	t, err := tail.TailFile(path, tail.Config{
		Follow:        true,
		Poll:          runtime.GOOS == "windows", // on Windows poll for file changes instead of using the default inotify
		Logger:        tail.DiscardingLogger,
		CompleteLines: true,
	})
	if err != nil {
		return nil, err
	}
	done := make(chan bool)
	go func(tailedLog *tail.Tail) {
		for line := range tailedLog.Lines {
			if line.Err != nil {
				for _, r := range receivers {
					r <- GrpcEntry{Error: line.Err}
				}
				continue
			}
			var e grpclog.GrpcLogEntry
			err = json.Unmarshal([]byte(line.Text), &e)
			if err != nil {
				for _, r := range receivers {
					r <- GrpcEntry{Error: err}
				}
				continue
			}
			for _, r := range receivers {
				r <- GrpcEntry{GrpcLogEntry: e}
			}
		}
		for _, r := range receivers {
			close(r)
		}
		close(done)
	}(t)
	return &fileWatcher{
		Filename:  t.Filename,
		tail:      t,
		receivers: receivers,
		done:      done,
	}, nil
}

func tailLogs(command string, receivers []chan<- GrpcEntry) (*fileWatcher, error) {
	logDir, err := os.MkdirTemp("", fmt.Sprintf("automation-logs-%s-", command))
	if err != nil {
		return nil, fmt.Errorf("failed to create logdir: %w", err)
	}
	logFile := filepath.Join(logDir, "grpclog.txt")

	t, err := watchFile(logFile, receivers)
	if err != nil {
		return nil, fmt.Errorf("failed to watch file: %w", err)
	}

	return t, nil
}

func (fw *fileWatcher) Close() {
	if fw.tail == nil {
		return
	}

	// Tell the watcher to end on next EoF, wait for the done event, then cleanup.
	//nolint:errcheck
	fw.tail.StopAtEOF()
	<-fw.done
	// logDir := filepath.Dir(fw.tail.Filename)
	fw.tail.Cleanup()
	// os.RemoveAll(logDir)

	// set to nil so we can safely close again in defer
	fw.tail = nil
}

type TypedEntry[TRequest any, TResponse any] struct {
	Request  TRequest
	Response TResponse
}

func unmarshalTypedEntry[TRequest, TResponse any](entry grpclog.GrpcLogEntry) (*TypedEntry[TRequest, TResponse], error) {
	reqSlot := new(TRequest)
	resSlot := new(TResponse)
	if err := jsonpb.Unmarshal([]byte(entry.Request), any(reqSlot).(protoreflect.ProtoMessage)); err != nil {
		return nil, err
	}
	if err := jsonpb.Unmarshal([]byte(entry.Response), any(resSlot).(protoreflect.ProtoMessage)); err != nil {
		return nil, err
	}
	typedEntry := TypedEntry[TRequest, TResponse]{
		Request:  *reqSlot,
		Response: *resSlot,
	}
	return &typedEntry, nil
}
