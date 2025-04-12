package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path"
	"runtime"
	"runtime/debug"

	"github.com/corymhall/pulumilsp/lsp"
	"github.com/corymhall/pulumilsp/rpc"
	"github.com/corymhall/pulumilsp/server"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

func main() {
	defer panicHandler()
	ctx := context.Background()
	logger := getLogger()
	stream := rpc.NewHeaderStream(os.Stdin, os.Stdout, logger)
	conn := rpc.NewConn(stream, logger)
	client := lsp.ClientDispatcher(conn)
	srv := server.New(logger, client)
	defer func() {
		if err := srv.Shutdown(ctx); err != nil {
			logger.Println("Error shutting down server:", err)
		}
	}()
	ctx = lsp.WithClient(ctx, client)
	go conn.Run(ctx, lsp.ServerHandler(srv, rpc.MethodNotFound))
	<-conn.Done()
}

func panicHandler() {
	if panicPayload := recover(); panicPayload != nil {
		stack := string(debug.Stack())
		fmt.Fprintln(os.Stderr, "================================================================================")
		fmt.Fprintln(os.Stderr, "Pulumi LSP encountered a fatal error. This is a bug!")
		fmt.Fprintln(os.Stderr, "We would appreciate a report: https://github.com/corymhall/pulumilsp/issues/")
		fmt.Fprintln(os.Stderr, "Please provide all of the below text in your report.")
		fmt.Fprintln(os.Stderr, "================================================================================")
		fmt.Fprintf(os.Stderr, "pulumilsp Version:   %s\n", "0.0.0") // TODO: Get the actual version
		fmt.Fprintf(os.Stderr, "Go Version:           %s\n", runtime.Version())
		fmt.Fprintf(os.Stderr, "Go Compiler:          %s\n", runtime.Compiler)
		fmt.Fprintf(os.Stderr, "Architecture:         %s\n", runtime.GOARCH)
		fmt.Fprintf(os.Stderr, "Operating System:     %s\n", runtime.GOOS)
		fmt.Fprintf(os.Stderr, "Panic:                %s\n\n", panicPayload)
		fmt.Fprintln(os.Stderr, stack)
		os.Exit(1)
	}
}

func getLogger() *log.Logger {
	home, err := os.UserHomeDir()
	contract.AssertNoErrorf(err, "could not find home directory")
	dir := path.Join(home, ".pulumilsp")
	err = os.MkdirAll(dir, 0700)
	contract.AssertNoErrorf(err, "could not create dir: %s", dir)
	filename := path.Join(dir, "log.txt")
	logfile, err := os.OpenFile(filename, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0666)
	contract.AssertNoErrorf(err, "failed to open log file: %s", filename)
	return log.New(logfile, "[pulumilsp]", log.Ldate|log.Ltime|log.Lshortfile)
}
