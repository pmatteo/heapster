package store

// Package main implements a simple Redis-like in-memory store in Go that supports
// RESP protocol and basic data structures: Lists, Hashes, and Sets. It is designed
// to be testable, modular, and idiomatic. Interfaces are used for abstractions so
// implementations can be swapped easily.

import (
	"fmt"
	"log/slog"
	"strings"

	"github.com/pmatteo/heapster/internal/commands"
)

// Store abstracts the data engine.
type Store interface {
	Execute(cmd string, args ...string) (any, error)
	RegisterFeature(cmd commands.Feature)
}

// InMemoryStore is a Redis-like in-memory store.
type InMemoryStore struct {
	maxItems int
	commands map[string]commands.Command
	logger   *slog.Logger
}

// NewInMemoryStore creates a new store with a maximum number of items allowed.
// If maxItems <= 0, the store grows unbounded.
func NewInMemoryStore(maxItems int, l *slog.Logger) *InMemoryStore {
	return &InMemoryStore{
		maxItems: maxItems,
		commands: make(map[string]commands.Command),
		logger:   l,
	}
}

// Execute finds the registered command and executes it.
func (ds *InMemoryStore) Execute(cmd string, args ...string) (any, error) {
	command, ok := ds.commands[strings.ToUpper(cmd)]

	if !ok {
		ds.logger.Error("unknown command", slog.String("cmd", cmd))
		return nil, fmt.Errorf("unknown command: %s", cmd)
	}
	return command.Execute(args...)
}

// RegisterFeature registers a command under a given name.
func (ds *InMemoryStore) RegisterFeature(feature commands.Feature) {
	for name, cmd := range feature {
		ds.commands[strings.ToUpper(name)] = cmd
	}
}
