package server

import (
	"log/slog"

	"github.com/pmatteo/heapster/internal/protocol/resp"
	"github.com/pmatteo/heapster/internal/store"
)

// NewRespServer creates a new RESP server with the provided components and middleware
func NewRespServer(
	store store.Store,
	baseHandler HandlerFunc,
	parser resp.Parser,
	encoder resp.Encoder,
	logger *slog.Logger,
	middleware ...Middleware,
) RespServer {
	h := baseHandler
	for i := len(middleware) - 1; i >= 0; i-- {
		h = middleware[i](h)
	}
	return &respServer{
		store:   store,
		parser:  parser,
		encoder: encoder,
		handler: h,
		logger:  logger,
		closing: make(chan struct{}),
	}
}

// NewDefaultRespServer creates a new RESP server with default configuration and logging
func NewDefaultRespServer(store store.Store, logger *slog.Logger) RespServer {

	mw := []Middleware{
		RecoverMiddleware(func(format string, args ...any) {
			logger.Error(format, args...)
		}),
		LoggingMiddleware(func(format string, args ...any) {
			logger.Info(format, args...)
		}),
	}

	return NewRespServer(
		store,
		baseHandlerFactory(store),
		resp.NewParserAdapter(logger),
		resp.NewEncoderAdapter(),
		logger,
		mw...,
	)
}
