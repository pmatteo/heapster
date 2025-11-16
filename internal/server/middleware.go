package server

import (
	"context"
	"fmt"
	"runtime/debug"
	"time"
)

func LoggingMiddleware(logf func(format string, args ...any)) Middleware {
	return func(next HandlerFunc) HandlerFunc {
		return func(ctx context.Context, req *Request) (any, error) {
			start := time.Now()
			resp, err := next(ctx, req)
			lat := time.Since(start)
			logf("handled request", "cmd", req.Cmd, "args", req.Args, "latency", lat, "error", err)
			return resp, err
		}
	}
}

func RecoverMiddleware(logf func(format string, args ...any)) Middleware {
	return func(next HandlerFunc) HandlerFunc {
		return func(ctx context.Context, req *Request) (res any, err error) {
			defer func() {
				if r := recover(); r != nil {
					stack := string(debug.Stack())
					logf("panic serving", "req.Cmd", req.Cmd, "req.Args", req.Args, "panic", r, "stack", stack)
					err = fmt.Errorf("internal error")
					res = nil
				}
			}()
			return next(ctx, req)
		}
	}
}
