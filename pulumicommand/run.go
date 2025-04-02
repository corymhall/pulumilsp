package pulumicommand

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"sync"

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

func run(ctx context.Context, stack auto.Stack, logger *log.Logger) (map[string]*ResourceInfo, error) {
	store := &ResourceStore{}
	events := make(chan GrpcEntry)

	go processGrpcEvents(ctx, events, store, logger)

	f, err := setupLogTailing("preview", events)
	if err != nil {
		return nil, fmt.Errorf("failed to tail logs: %w", err)
	}
	defer func() {
		close(events)
		f.Close()
	}()

	logger.Printf("Tailing logs from file: %s", f.Filename)
	stack.Workspace().SetEnvVar("PULUMI_DEBUG_GRPC", f.Filename)

	_, err = stack.Preview(ctx, PolicyPacks([]string{"/Users/chall/work/tmp/policypack"}), optpreview.SuppressProgress())
	return store.Resources, err
}

func processGrpcEvents(ctx context.Context, events <-chan GrpcEntry, store *ResourceStore, logger *log.Logger) {
	select {
	case <-ctx.Done():
		logger.Println("Stopping processGrpcEvents due to context cancellation")
		return
	case evt, ok := <-events:
		if !ok {
			logger.Println("Events channel closed, stopping processGrpcEvents")
			return
		}
		handleGrpcEvent(evt, store, logger)
	}
}

func handleGrpcEvent(evt GrpcEntry, store *ResourceStore, logger *log.Logger) {
	switch evt.Method {
	case "/pulumirpc.ResourceMonitor/RegisterResource":
		handleRegisterResource(evt, store, logger)
	case "/pulumirpc.Analyzer/AnalyzeStack":
		handleAnalyzeStack(evt, store, logger)
	case "/pulumirpc.Analyzer/Analyze":
		handleAnalyze(evt, store, logger)
	default:
		// Unhandled method
	}
}

func handleRegisterResource(evt GrpcEntry, store *ResourceStore, logger *log.Logger) {
	tEntry, err := unmarshalTypedEntry[rpc.RegisterResourceRequest, rpc.RegisterResourceResponse](evt.GrpcLogEntry)
	if err != nil {
		logger.Printf("Error unmarshalling register resource entry: %v", err)
		return
	}
	store.getOrCreateResourceInfo(tEntry.Response.Urn).SetSourcePosition(tEntry.Request.SourcePosition)
}

func handleAnalyzeStack(evt GrpcEntry, store *ResourceStore, logger *log.Logger) {
	tEntry, err := unmarshalTypedEntry[rpc.AnalyzeStackRequest, rpc.AnalyzeResponse](evt.GrpcLogEntry)
	if err != nil {
		logger.Printf("Error unmarshalling analyze stack entry: %v", err)
		return
	}
	if tEntry.Response.Diagnostics == nil {
		logger.Printf("No diagnostics found in analyze stack response for Stack")
		return
	}
	for _, d := range tEntry.Response.Diagnostics {
		store.getOrCreateResourceInfo(d.Urn).AddDiagnostic(d)
	}
}

func handleAnalyze(evt GrpcEntry, store *ResourceStore, logger *log.Logger) {
	tEntry, err := unmarshalTypedEntry[rpc.AnalyzeRequest, rpc.AnalyzeResponse](evt.GrpcLogEntry)
	if err != nil {
		logger.Printf("Error unmarshalling analyze entry: %v", err)
		return
	}
	if tEntry.Response.Diagnostics == nil {
		logger.Printf("No diagnostics found in analyze response for URN: %s", tEntry.Request.Urn)
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
		defer func() {
			for _, r := range receivers {
				close(r)
			}
			close(done)
		}()
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
