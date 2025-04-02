package lsp

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"

	"github.com/corymhall/pulumilsp/rpc"
	"github.com/corymhall/pulumilsp/xcontext"
)

type contextKey int

type ProgressToken any

const (
	clientKey = contextKey(iota)
)

// UnmarshalJSON unmarshals msg into the variable pointed to by
// params. In JSONRPC, optional messages may be
// "null", in which case it is a no-op.
func UnmarshalJSON(msg json.RawMessage, v any) error {
	if len(msg) == 0 || bytes.Equal(msg, []byte("null")) {
		return nil
	}
	return json.Unmarshal(msg, v)
}

func WithClient(ctx context.Context, client Client) context.Context {
	return context.WithValue(ctx, clientKey, client)
}

var (
	// RequestCancelledError should be used when a request is cancelled early.
	RequestCancelledError = errors.New("JSON RPC cancelled")
)

type connSender interface {
	Notify(ctx context.Context, method string, params any) error
	Call(ctx context.Context, method string, params, result any) error
}

type clientDispatcher struct {
	sender connSender
}

func ClientDispatcher(conn rpc.Conn) Client {
	return &clientDispatcher{
		sender: clientConn{conn},
	}
}

type clientConn struct {
	conn rpc.Conn
}

func (c clientConn) Notify(ctx context.Context, method string, params any) error {
	return c.conn.Notify(ctx, method, params)
}

func (c clientConn) Call(ctx context.Context, method string, params any, result any) error {
	c.conn.Logger().Printf("Calling method: %s", method)
	id, err := c.conn.Call(ctx, method, params, result)
	if ctx.Err() != nil {
		c.conn.Logger().Printf("Request cancelled: %s", method)
		cancelCall(ctx, c, id)
	}
	return err
}

// See https://microsoft.github.io/language-server-protocol/specifications/lsp/3.17/specification#cancelParams
type CancelParams struct {
	// The request id to cancel.
	ID any `json:"id"`
}

func cancelCall(ctx context.Context, sender connSender, id any) {
	ctx = xcontext.Detach(ctx)
	_ = sender.Notify(ctx, "$/cancelRequest", &CancelParams{ID: &id})
}

func ServerHandler(server Server, handler rpc.Handler) rpc.Handler {
	return func(ctx context.Context, reply rpc.Replier, req rpc.Request) error {
		if ctx.Err() != nil {
			ctx := xcontext.Detach(ctx)
			return reply(ctx, nil, RequestCancelledError)
		}
		handled, err := serverDispatch(ctx, server, reply, req)
		if handled || err != nil {
			return err
		}
		return handler(ctx, reply, req)
	}
}
