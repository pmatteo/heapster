package hash

import (
	"fmt"

	"github.com/pmatteo/heapster/internal/commands"
	"github.com/puzpuzpuz/xsync/v4"
)

// Hash provides a thread-safe hash data structure using xsync for concurrent access
type Hash struct {
	data *xsync.Map[string, *xsync.Map[string, string]]
}

// NewVersion creates a new Hash instance
func NewVersion() *Hash {
	return &Hash{
		data: xsync.NewMap[string, *xsync.Map[string, string]](),
	}
}

// NewFeature creates a new Feature with hash commands
func NewFeature() commands.Feature {
	h := NewVersion()

	return commands.Feature{
		"HSET": &HSetCommand{H: h},
		"HGET": &HGetCommand{H: h},
	}
}

type HSetCommand struct{ H *Hash }

// Execute sets the value of a field in the hash stored at key
func (c *HSetCommand) Execute(args ...string) (any, error) {
	if len(args) < 3 {
		return nil, fmt.Errorf("HSET requires key field value")
	}
	key, field, value := args[0], args[1], args[2]

	// Get or create the inner map for this key
	innerMap, _ := c.H.data.LoadOrCompute(key, func() (*xsync.Map[string, string], bool) {
		return xsync.NewMap[string, string](), false
	})

	// Store the field-value pair
	innerMap.Store(field, value)
	return int64(1), nil
}

type HGetCommand struct{ H *Hash }

// Execute retrieves the value of a field from the hash stored at key
func (c *HGetCommand) Execute(args ...string) (any, error) {
	l := len(args)
	if l < 1 {
		return nil, fmt.Errorf("HSET key field value [field value ...] requires at least key field")
	}

	// Try to load the inner map for this key
	innerMap, ok := c.H.data.Load(args[0])
	if !ok {
		return nil, nil
	}

	if l == 2 {
		if value, ok := innerMap.Load(args[1]); ok {
			return value, nil
		}
		return nil, nil
	}

	result := make([]string, 0, innerMap.Size()*2)
	innerMap.Range(func(field string, value string) bool {
		if field != "" && value != "" {
			result = append(result, field, value)
		}
		return true
	})

	return result, nil
}
