package server

import (
	"context"

	"github.com/pmatteo/heapster/internal/store"
)

// Request represents a parsed RESP command.
type Request struct {
	Cmd  string
	Args []string
}

// HandlerFunc processes a single Request.
type HandlerFunc func(ctx context.Context, req *Request) (any, error)

// Middleware wraps a HandlerFunc for cross-cutting concerns.
type Middleware func(next HandlerFunc) HandlerFunc

func baseHandlerFactory(store store.Store) HandlerFunc {
	return func(ctx context.Context, req *Request) (any, error) {
		return store.Execute(req.Cmd, req.Args...)
	}
}
