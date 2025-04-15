package lsp

import (
	"context"
)

type contextKey int

const (
	clientKey = contextKey(iota)
)

func WithClient(ctx context.Context, client Client) context.Context {
	return context.WithValue(ctx, clientKey, client)
}

func GetClient(ctx context.Context) Client {
	client, ok := ctx.Value(clientKey).(Client)
	if !ok {
		return nil
	}
	return client
}
