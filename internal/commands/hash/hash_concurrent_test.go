package hash

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/puzpuzpuz/xsync/v4"
	"github.com/stretchr/testify/assert"
)

const (
	// Test configuration constants
	defaultGoroutines      = 100
	defaultOperations      = 1000
	defaultOpsPerGoroutine = 100
	defaultReaders         = 50
	defaultWorkers         = 50
	stressTestWorkers      = 20
	stressTestOperations   = 500
	prePopulateSize        = 100
	keysModulo             = 10
	fieldsModulo           = 100
)

// TestXHashConcurrentWrites tests concurrent write operations on inner map
func TestXHashConcurrentWrites(t *testing.T) {
	t.Parallel()

	h := NewVersion()
	innerMap, _ := h.data.LoadOrCompute("testkey", func() (*xsync.Map[string, string], bool) {
		return xsync.NewMap[string, string](), false
	})

	var wg sync.WaitGroup
	wg.Add(defaultGoroutines)

	for i := range defaultGoroutines {
		go func(id int) {
			defer wg.Done()
			for j := range defaultOpsPerGoroutine {
				field := fmt.Sprintf("field_%d_%d", id, j)
				value := fmt.Sprintf("value_%d_%d", id, j)
				innerMap.Store(field, value)
			}
		}(i)
	}

	wg.Wait()

	// Verify all values were written correctly
	for i := range defaultGoroutines {
		for j := range defaultOpsPerGoroutine {
			field := fmt.Sprintf("field_%d_%d", i, j)
			expectedValue := fmt.Sprintf("value_%d_%d", i, j)
			val, ok := innerMap.Load(field)
			assert.True(t, ok, "field %s should exist after concurrent writes (goroutine %d, operation %d)", field, i, j)
			assert.Equal(t, expectedValue, val, "field %s should have correct value after concurrent writes (goroutine %d, operation %d)", field, i, j)
		}
	}

	// Verify total count
	expectedCount := defaultGoroutines * defaultOpsPerGoroutine
	actualCount := innerMap.Size()
	assert.Equal(t, expectedCount, actualCount, "hash table should contain %d entries after concurrent writes, got %d", expectedCount, actualCount)
}

// TestXHashConcurrentReadsAndWrites tests concurrent read and write operations
func TestXHashConcurrentReadsAndWrites(t *testing.T) {
	t.Parallel()

	h := NewVersion()
	innerMap, _ := h.data.LoadOrCompute("testkey", func() (*xsync.Map[string, string], bool) {
		return xsync.NewMap[string, string](), false
	})

	// Pre-populate with initial data
	for i := range defaultOperations {
		innerMap.Store(fmt.Sprintf("field%d", i), fmt.Sprintf("value%d", i))
	}

	var wg sync.WaitGroup
	wg.Add(defaultWorkers * 2)

	// Start writer goroutines
	for i := range defaultWorkers {
		go func(id int) {
			defer wg.Done()
			for j := range defaultOperations {
				field := fmt.Sprintf("field%d", (id*defaultOperations+j)%defaultOperations)
				value := fmt.Sprintf("value_%d_%d", id, j)
				innerMap.Store(field, value)
			}
		}(i)
	}

	// Start reader goroutines
	for i := range defaultWorkers {
		go func(id int) {
			defer wg.Done()
			for j := range defaultOperations {
				field := fmt.Sprintf("field%d", j%fieldsModulo)
				_, _ = innerMap.Load(field)
			}
		}(i)
	}

	wg.Wait()

	// Verify hash table is still functional
	actualCount := innerMap.Size()
	assert.NotZero(t, actualCount, "hash table should not be empty after concurrent reads and writes")
	assert.GreaterOrEqual(t, actualCount, fieldsModulo, "hash table should contain at least %d entries after concurrent operations", fieldsModulo)
}

// TestXHashConcurrentGetAll tests concurrent Range operations
func TestXHashConcurrentGetAll(t *testing.T) {
	t.Parallel()

	h := NewVersion()
	innerMap, _ := h.data.LoadOrCompute("testkey", func() (*xsync.Map[string, string], bool) {
		return xsync.NewMap[string, string](), false
	})

	// Pre-populate hash table
	for i := range prePopulateSize {
		innerMap.Store(fmt.Sprintf("field%d", i), fmt.Sprintf("value%d", i))
	}

	var wg sync.WaitGroup
	wg.Add(defaultReaders)

	for range defaultReaders {
		go func() {
			defer wg.Done()
			for j := range prePopulateSize {
				result := make([]string, 0, prePopulateSize*2)
				innerMap.Range(func(k, v string) bool {
					result = append(result, k, v)
					return true
				})
				assert.NotEmpty(t, result, "Range should return non-empty result on iteration %d", j)
				assert.Len(t, result, prePopulateSize*2, "Range should return %d elements (field-value pairs) on iteration %d, got %d", prePopulateSize*2, j, len(result))
			}
		}()
	}

	wg.Wait()
}

// TestXHashConcurrentHSetDifferentKeys tests concurrent HSET operations on different keys
func TestXHashConcurrentHSetDifferentKeys(t *testing.T) {
	t.Parallel()

	h := NewVersion()
	cmd := &HSetCommand{H: h}
	var wg sync.WaitGroup
	wg.Add(defaultGoroutines)

	for i := range defaultGoroutines {
		go func(id int) {
			defer wg.Done()
			key := fmt.Sprintf("key%d", id)
			for j := range defaultOpsPerGoroutine {
				field := fmt.Sprintf("field%d", j)
				value := fmt.Sprintf("value%d", j)
				_, err := cmd.Execute(key, field, value)
				assert.NoError(t, err, "HSET should succeed for key '%s', field %s (goroutine %d, operation %d)", key, field, id, j)
			}
		}(i)
	}

	wg.Wait()

	// Count keys
	actualKeys := 0
	h.data.Range(func(_ string, _ *xsync.Map[string, string]) bool {
		actualKeys++
		return true
	})

	assert.Equal(t, defaultGoroutines, actualKeys, "hash should contain %d keys after concurrent HSET on different keys, got %d", defaultGoroutines, actualKeys)
}

// TestXHashConcurrentHSetSameKey tests concurrent HSET operations on the same key
func TestXHashConcurrentHSetSameKey(t *testing.T) {
	t.Parallel()

	h := NewVersion()
	cmd := &HSetCommand{H: h}
	const sharedKey = "sharedkey"

	var wg sync.WaitGroup
	wg.Add(defaultGoroutines)

	for i := range defaultGoroutines {
		go func(id int) {
			defer wg.Done()
			for j := range defaultOpsPerGoroutine {
				field := fmt.Sprintf("field_%d_%d", id, j)
				value := fmt.Sprintf("value_%d_%d", id, j)
				_, err := cmd.Execute(sharedKey, field, value)
				assert.NoError(t, err, "HSET should succeed for shared key %s, field %s (goroutine %d, operation %d)", sharedKey, field, id, j)
			}
		}(i)
	}

	wg.Wait()

	innerMap, ok := h.data.Load(sharedKey)
	assert.True(t, ok, "shared key %s should exist in hash after concurrent HSET operations", sharedKey)

	expectedCount := defaultGoroutines * defaultOpsPerGoroutine
	actualCount := innerMap.Size()
	assert.Equal(t, expectedCount, actualCount, "shared key %s should contain %d fields after concurrent HSET, got %d", sharedKey, expectedCount, actualCount)
}

// TestXHashConcurrentHSetAndHGet tests concurrent HSET and HGET operations
func TestXHashConcurrentHSetAndHGet(t *testing.T) {
	t.Parallel()

	h := NewVersion()
	setCmd := &HSetCommand{H: h}
	getCmd := &HGetCommand{H: h}

	var wg sync.WaitGroup
	wg.Add(defaultWorkers * 2)

	// Writer goroutines
	for i := range defaultWorkers {
		go func(id int) {
			defer wg.Done()
			for j := range defaultOperations {
				key := fmt.Sprintf("key%d", id%keysModulo)
				field := fmt.Sprintf("field%d", j)
				value := fmt.Sprintf("value_%d_%d", id, j)
				_, err := setCmd.Execute(key, field, value)
				assert.NoError(t, err, "HSET should succeed for key '%s', field %s (writer %d, operation %d)", key, field, id, j)
			}
		}(i)
	}

	// Reader goroutines
	for i := range defaultWorkers {
		go func(id int) {
			defer wg.Done()
			for j := range defaultOperations {
				key := fmt.Sprintf("key%d", id%keysModulo)
				field := fmt.Sprintf("field%d", j)
				_, err := getCmd.Execute(key, field)
				assert.NoError(t, err, "HGET should not return error for key '%s', field %s (reader %d, operation %d)", key, field, id, j)
			}
		}(i)
	}

	wg.Wait()

	// Verify hash contains expected keys
	actualKeys := 0
	h.data.Range(func(_ string, _ *xsync.Map[string, string]) bool {
		actualKeys++
		return true
	})

	assert.LessOrEqual(t, actualKeys, keysModulo, "hash should contain at most %d keys after concurrent operations, got %d", keysModulo, actualKeys)
	assert.Greater(t, actualKeys, 0, "hash should contain at least one key after concurrent operations")
}

// TestXHashStressWithResize tests concurrent operations (xsync handles resizing automatically)
func TestXHashStressWithResize(t *testing.T) {
	t.Parallel()

	h := NewVersion()
	setCmd := &HSetCommand{H: h}
	getCmd := &HGetCommand{H: h}

	var wg sync.WaitGroup
	wg.Add(stressTestWorkers * 2)

	// Heavy writer goroutines
	for i := range stressTestWorkers {
		go func(id int) {
			defer wg.Done()
			key := fmt.Sprintf("key%d", id%5)
			for j := range stressTestOperations {
				field := fmt.Sprintf("field_%d_%d", id, j)
				value := fmt.Sprintf("value_%d_%d", id, j)
				_, err := setCmd.Execute(key, field, value)
				assert.NoError(t, err, "HSET should succeed during stress test for key '%s', field %s (writer %d, operation %d)", key, field, id, j)
			}
		}(i)
	}

	// Reader goroutines
	for i := range stressTestWorkers {
		go func(id int) {
			defer wg.Done()
			for range stressTestOperations {
				key := fmt.Sprintf("key%d", id%5)
				_, err := getCmd.Execute(key)
				assert.NoError(t, err, "HGET should not return error during stress test for key '%s' (reader %d)", key, id)
				time.Sleep(time.Microsecond)
			}
		}(i)
	}

	wg.Wait()

	// Verify hash integrity after stress test
	actualKeys := 0
	h.data.Range(func(_ string, _ *xsync.Map[string, string]) bool {
		actualKeys++
		return true
	})

	assert.LessOrEqual(t, actualKeys, 5, "hash should contain at most 5 keys after stress test, got %d", actualKeys)
	assert.Greater(t, actualKeys, 0, "hash should contain at least one key after stress test")

	// Verify each key's hash table has entries
	h.data.Range(func(key string, innerMap *xsync.Map[string, string]) bool {
		size := innerMap.Size()
		assert.Greater(t, size, 0, "key '%s' should have at least one field after stress test", key)
		return true
	})
}

// FuzzXHashConcurrent fuzzes concurrent operations
func FuzzXHashConcurrent(f *testing.F) {
	f.Add("field1", "value1", "field2", "value2")

	f.Fuzz(func(t *testing.T, field1, value1, field2, value2 string) {
		h := NewVersion()
		innerMap, _ := h.data.LoadOrCompute("testkey", func() (*xsync.Map[string, string], bool) {
			return xsync.NewMap[string, string](), false
		})

		var wg sync.WaitGroup
		wg.Add(4)

		// Writer 1
		go func() {
			defer wg.Done()
			innerMap.Store(field1, value1)
		}()

		// Writer 2
		go func() {
			defer wg.Done()
			innerMap.Store(field2, value2)
		}()

		// Reader 1
		go func() {
			defer wg.Done()
			innerMap.Load(field1)
		}()

		// Reader 2 (Range)
		go func() {
			defer wg.Done()
			innerMap.Range(func(_, _ string) bool {
				return true
			})
		}()

		wg.Wait()

		// Verify final state
		if val, ok := innerMap.Load(field1); ok {
			assert.Equal(t, value1, val, "field1 should have value %s after concurrent operations, got %s", value1, val)
		}
		if val, ok := innerMap.Load(field2); ok {
			assert.Equal(t, value2, val, "field2 should have value %s after concurrent operations, got %s", value2, val)
		}
	})
}
