package hash

import (
	"fmt"
	"math/rand"
	"testing"

	"github.com/puzpuzpuz/xsync/v4"
)

// BenchmarkXHashSet benchmarks direct XHash set operations
func BenchmarkXHashSet(b *testing.B) {
	h := NewVersion()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		key := fmt.Sprintf("key%d", i%100)
		field := fmt.Sprintf("field%d", i)
		value := fmt.Sprintf("value%d", i)
		inner, _ := h.data.LoadOrCompute(key, func() (*xsync.Map[string, string], bool) {
			return xsync.NewMap[string, string](), false
		})
		inner.Store(field, value)
	}
}

// BenchmarkXHashGet benchmarks direct XHash get operations
func BenchmarkXHashGet(b *testing.B) {
	h := NewVersion()

	// Pre-populate
	for i := 0; i < 10000; i++ {
		key := fmt.Sprintf("key%d", i%100)
		field := fmt.Sprintf("field%d", i)
		value := fmt.Sprintf("value%d", i)
		inner, _ := h.data.LoadOrCompute(key, func() (*xsync.Map[string, string], bool) {
			return xsync.NewMap[string, string](), false
		})
		inner.Store(field, value)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		key := fmt.Sprintf("key%d", i%100)
		field := fmt.Sprintf("field%d", i%10000)

		if inner, ok := h.data.Load(key); ok {
			_, _ = inner.Load(field)
		}
	}
}

// BenchmarkXHashConcurrent benchmarks concurrent direct XHash ops
func BenchmarkXHashConcurrent(b *testing.B) {
	h := NewVersion()

	b.RunParallel(func(pb *testing.PB) {
		id := rand.Intn(1_000_000)
		i := 0
		for pb.Next() {
			key := fmt.Sprintf("key_%d", id)

			if i%2 == 0 {
				field := fmt.Sprintf("field_%d", i)
				value := fmt.Sprintf("value_%d", i)
				inner, _ := h.data.LoadOrCompute(key, func() (*xsync.Map[string, string], bool) {
					return xsync.NewMap[string, string](), false
				})
				inner.Store(field, value)
			} else {
				field := fmt.Sprintf("field_%d", i-1)
				if inner, ok := h.data.Load(key); ok {
					inner.Load(field)
				}
			}
			i++
		}
	})
}

// BenchmarkXHSetCommand benchmarks XHSET command usage
func BenchmarkXHSetCommand(b *testing.B) {
	h := NewVersion()
	cmd := &HSetCommand{H: h}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		key := fmt.Sprintf("key%d", i%100)
		field := fmt.Sprintf("field%d", i)
		value := fmt.Sprintf("value%d", i)
		_, _ = cmd.Execute(key, field, value)
	}
}

// BenchmarkXHGetCommand benchmarks XHGET command usage
func BenchmarkXHGetCommand(b *testing.B) {
	h := NewVersion()
	setCmd := &HSetCommand{H: h}
	getCmd := &HGetCommand{H: h}

	// Pre-populate
	for i := 0; i < 10000; i++ {
		key := fmt.Sprintf("key%d", i%100)
		field := fmt.Sprintf("field%d", i)
		value := fmt.Sprintf("value%d", i)
		_, _ = setCmd.Execute(key, field, value)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		key := fmt.Sprintf("key%d", i%100)
		field := fmt.Sprintf("field%d", i%10000)
		_, _ = getCmd.Execute(key, field)
	}
}
