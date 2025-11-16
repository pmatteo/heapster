package list

import (
	"fmt"

	"github.com/pmatteo/heapster/internal/commands"
	"github.com/puzpuzpuz/xsync/v4"
)

// List implements Redis-like list commands using xsync for concurrent access
type List struct {
	data *xsync.Map[string, []string]
}

// NewFeature creates a new Feature with list commands
func NewFeature() commands.Feature {
	l := &List{
		data: xsync.NewMap[string, []string](),
	}

	return commands.Feature{
		"LPUSH":  &LPushCommand{L: l},
		"LRANGE": &LRangeCommand{L: l},
	}
}

// LPushCommand key values
type LPushCommand struct{ L *List }

// Execute prepends one or multiple values to a list
func (c *LPushCommand) Execute(args ...string) (any, error) {
	if len(args) < 2 {
		return nil, fmt.Errorf("LPUSH requires key and values")
	}
	key := args[0]
	values := args[1:]

	// Compute new list by prepending values
	newList, _ := c.L.data.Compute(key, func(oldList []string, loaded bool) ([]string, xsync.ComputeOp) {
		if !loaded {
			// No existing list, just return the values
			return values, xsync.CancelOp
		}
		// Prepend new values to existing list
		result := make([]string, 0, len(values)+len(oldList))
		result = append(result, values...)
		result = append(result, oldList...)
		return result, xsync.UpdateOp
	})

	return int64(len(newList)), nil
}

// LRangeCommand key start stop
type LRangeCommand struct{ L *List }

// Execute retrieves a range of elements from a list
func (c *LRangeCommand) Execute(args ...string) (any, error) {
	if len(args) != 3 {
		return nil, fmt.Errorf("LRANGE requires key, start, stop")
	}
	key := args[0]
	// naive parse
	var start, stop int
	fmt.Sscanf(args[1], "%d", &start)
	fmt.Sscanf(args[2], "%d", &stop)

	list, ok := c.L.data.Load(key)
	if !ok {
		return []string{}, nil
	}

	if start < 0 {
		start = 0
	}
	if stop >= len(list) {
		stop = len(list) - 1
	}
	if stop < start {
		return []string{}, nil
	}
	return list[start : stop+1], nil
}
