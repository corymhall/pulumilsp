package rpc

import (
	"context"
	"errors"
	"fmt"
)

var (
	// ErrUnknown should be used for all non coded errors.
	ErrUnknown = "JSON RPC unknown error"
	// ErrParse is used when invalid JSON was received by the server.
	ErrParse = "JSON RPC parse error"
	//ErrInvalidRequest is used when the JSON sent is not a valid Request object.
	ErrInvalidRequest = "JSON RPC invalid request"
	// ErrMethodNotFound should be returned by the handler when the method does
	// not exist / is not available.
	ErrMethodNotFound = "JSON RPC method not found"
	// ErrInvalidParams should be returned by the handler when method
	// parameter(s) were invalid.
	ErrInvalidParams = "JSON RPC invalid params"
	// ErrInternal is not currently returned but defined for completeness.
	ErrInternal = "JSON RPC internal error"

	//ErrServerOverloaded is returned when a message was refused due to a
	//server being temporarily unable to accept any new messages.
	ErrServerOverloaded = "JSON RPC overloaded"
)

// Handler is invoked to handle incoming requests.
// The Replier sends a reply to the request and must be called exactly once.
type Handler func(ctx context.Context, reply Replier, req Request) error

// Replier is passed to handlers to allow them to reply to the request.
// If err is set then result will be ignored.
type Replier func(ctx context.Context, result any, err error) error

// MethodNotFound is a Handler that replies to all call requests with the
// standard method not found response.
// This should normally be the final handler in a chain.
func MethodNotFound(ctx context.Context, reply Replier, req Request) error {
	return reply(ctx, nil, fmt.Errorf("%w: %q", errors.New(ErrMethodNotFound), req.Method()))
}
