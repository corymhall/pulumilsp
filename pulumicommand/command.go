package pulumicommand

import (
	"context"
	"sync"

	"github.com/corymhall/pulumilsp/debug"
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

func (r *Runner) Run(ctx context.Context) (map[string]*ResourceInfo, error) {
	r.initialize()
	ctx, done := debug.Start(ctx, "pulumicommand.Run")
	defer done()

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
		return nil, ctx.Err()
	case r.inFlight <- struct{}{}:
		defer func() { <-r.inFlight }()
	}
	res, err := run(ctx, r.stack)
	if err != nil {
		debug.LogError(ctx, "error running pulumi command", err)
	}
	return res, nil
}
