package set

import (
	"fmt"
	"log/slog"

	"github.com/pmatteo/heapster/internal/commands"
	"github.com/puzpuzpuz/xsync/v4"
)

// Set represents a thread-safe set data structure using xsync's concurrent map
type Set struct {
	// data maps keys to their corresponding set of values
	// The inner map uses struct{} as values for memory efficiency
	data *xsync.Map[string, *xsync.Map[string, any]]
}

// NewFeature creates and returns a new Set feature with SADD and SMEMBERS commands
func NewFeature(l *slog.Logger) commands.Feature {
	set := &Set{
		data: xsync.NewMap[string, *xsync.Map[string, any]](),
	}

	return commands.Feature{
		"SADD":     &SAddCommand{S: set, logger: l},
		"SMEMBERS": &SMembersCommand{S: set, logger: l},
	}
}

type SAddCommand struct {
	S      *Set
	logger *slog.Logger
}

// Execute adds one or more values to a set stored at key
// Returns the number of elements that were added (excluding duplicates)
func (c *SAddCommand) Execute(args ...string) (any, error) {
	if len(args) < 2 {
		return nil, fmt.Errorf("SADD requires key value")
	}
	key := args[0]
	values := args[1:]

	c.logger.Info("ADD ", "key", key, "values", values)

	// Get or create the set for this key
	set, _ := c.S.data.LoadOrCompute(key, func() (*xsync.Map[string, any], bool) {
		return xsync.NewMap[string, any](), false
	})

	added := 0
	for _, v := range values {
		// Store returns true if the value was newly inserted
		if _, loaded := set.LoadOrStore(v, struct{}{}); !loaded {
			added++
		}
	}

	return "OK", nil
}

type SMembersCommand struct {
	S      *Set
	logger *slog.Logger
}

// Execute returns all members of the set stored at key
// Returns an empty slice if the key doesn't exist
func (c *SMembersCommand) Execute(args ...string) (any, error) {
	if len(args) < 1 {
		return nil, fmt.Errorf("SMEMBERS requires key")
	}
	key := args[0]

	c.logger.Info("SMEMBERS ", "key", key)

	members := []string{}

	// Load the set for this key
	if set, ok := c.S.data.Load(key); ok {
		// Iterate over all values in the set
		set.Range(func(v string, _ any) bool {
			members = append(members, v)
			return true // continue iteration
		})
	}

	return members, nil
}
