package pulumicommand

import (
	"context"
	"log"
	"sync"

	"github.com/pulumi/pulumi/sdk/v3/go/auto"
)

type Runner struct {
	once sync.Once

	inFlight chan struct{}

	serialized chan struct{}

	stack auto.Stack
}

func New(stack auto.Stack) *Runner {
	return &Runner{
		stack: stack,
	}
}

func (r *Runner) initialize() {
	r.once.Do(func() {
		r.inFlight = make(chan struct{}, 1)
		r.serialized = make(chan struct{}, 1)
	})
}

func (r *Runner) Run(ctx context.Context, logger *log.Logger) (map[string]*ResourceInfo, error) {
	r.initialize()

	// Acquire the serialization lock.
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case r.serialized <- struct{}{}:
		defer func() { <-r.serialized }()
	}

	// Wait for in-progress pulumi commands to return before proceeding
	select {
	case <-ctx.Done():
		logger.Println("context done")
		return nil, ctx.Err()
	case r.inFlight <- struct{}{}:
		defer func() { <-r.inFlight }()
	}
	logger.Printf("Running pulumi command...")
	res, err := run(ctx, r.stack, logger)
	if err != nil {
		logger.Printf("error running pulumi command: %v", err)
	}
	return res, nil
}
