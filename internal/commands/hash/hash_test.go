package hash

import (
	"testing"

	"github.com/puzpuzpuz/xsync/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

//
// BASIC HASH TESTS
//

func TestXHashBasicOperations(t *testing.T) {
	t.Parallel()

	t.Run("set and get single value", func(t *testing.T) {
		t.Parallel()
		h := NewVersion()

		root, _ := h.data.LoadOrCompute("root", func() (*xsync.Map[string, string], bool) {
			return xsync.NewMap[string, string](), false
		})

		root.Store("field1", "value1")

		val, ok := root.Load("field1")
		assert.True(t, ok)
		assert.Equal(t, "value1", val)
	})

	t.Run("update existing value", func(t *testing.T) {
		t.Parallel()
		h := NewVersion()
		root, _ := h.data.LoadOrCompute("root", func() (*xsync.Map[string, string], bool) {
			return xsync.NewMap[string, string](), false
		})

		root.Store("field1", "value1")
		root.Store("field1", "value2")

		val, ok := root.Load("field1")
		assert.True(t, ok)
		assert.Equal(t, "value2", val)
	})

	t.Run("get non-existent field", func(t *testing.T) {
		t.Parallel()
		h := NewVersion()
		root, _ := h.data.LoadOrCompute("root", func() (*xsync.Map[string, string], bool) {
			return xsync.NewMap[string, string](), false
		})

		val, ok := root.Load("nonexistent")
		assert.False(t, ok)
		assert.Equal(t, "", val)
	})

	t.Run("multiple fields", func(t *testing.T) {
		t.Parallel()
		h := NewVersion()
		root, _ := h.data.LoadOrCompute("root", func() (*xsync.Map[string, string], bool) {
			return xsync.NewMap[string, string](), false
		})

		fields := map[string]string{
			"field1": "value1",
			"field2": "value2",
			"field3": "value3",
		}

		for k, v := range fields {
			root.Store(k, v)
		}

		for k, expected := range fields {
			val, ok := root.Load(k)
			assert.True(t, ok)
			assert.Equal(t, expected, val)
		}
	})
}

//
// GET ALL TEST
//

func TestXHashGetAll(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		fields map[string]string
	}{
		{name: "empty", fields: map[string]string{}},
		{name: "single", fields: map[string]string{"field1": "value1"}},
		{
			name: "multiple",
			fields: map[string]string{
				"field1": "value1",
				"field2": "value2",
				"field3": "value3",
			},
		},
		{
			name: "many",
			fields: map[string]string{
				"field1":  "value1",
				"field2":  "value2",
				"field3":  "value3",
				"field4":  "value4",
				"field5":  "value5",
				"field6":  "value6",
				"field7":  "value7",
				"field8":  "value8",
				"field9":  "value9",
				"field10": "value10",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			h := NewVersion()
			root, _ := h.data.LoadOrCompute("root", func() (*xsync.Map[string, string], bool) {
				return xsync.NewMap[string, string](), false
			})

			for k, v := range tt.fields {
				root.Store(k, v)
			}

			var flat []string
			root.Range(func(k, v string) bool {
				flat = append(flat, k, v)
				return true
			})

			if len(tt.fields) == 0 {
				assert.Empty(t, flat)
				return
			}

			assert.Len(t, flat, len(tt.fields)*2)

			resultMap := make(map[string]string)
			for i := 0; i < len(flat); i += 2 {
				resultMap[flat[i]] = flat[i+1]
			}

			assert.Equal(t, tt.fields, resultMap)
		})
	}
}

//
// HSET TESTS
//

func TestXHSetCommand(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		args      []string
		setup     func(*Hash)
		wantErr   bool
		errMsg    string
		wantValue int64
	}{
		{
			name:      "valid HSET new field",
			args:      []string{"mykey", "field1", "value1"},
			wantValue: 1,
		},
		{
			name: "update existing field",
			args: []string{"mykey", "field1", "value2"},
			setup: func(h *Hash) {
				cmd := &HSetCommand{H: h}
				cmd.Execute("mykey", "field1", "value1")
			},
			wantValue: 1,
		},
		{
			name:    "missing value",
			args:    []string{"mykey", "field1"},
			wantErr: true,
		},
		{
			name:    "missing field",
			args:    []string{"mykey"},
			wantErr: true,
		},
		{
			name:    "missing all",
			args:    []string{},
			wantErr: true,
		},
		{
			name:      "empty key allowed",
			args:      []string{"", "field1", "value1"},
			wantValue: 1,
		},
		{
			name:      "empty field allowed",
			args:      []string{"mykey", "", "value1"},
			wantValue: 1,
		},
		{
			name:      "empty value allowed",
			args:      []string{"mykey", "field1", ""},
			wantValue: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			h := NewVersion()
			if tt.setup != nil {
				tt.setup(h)
			}

			cmd := &HSetCommand{H: h}
			result, err := cmd.Execute(tt.args...)

			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.wantValue, result)
		})
	}

	t.Run("multiple keys", func(t *testing.T) {
		t.Parallel()
		h := NewVersion()
		cmd := &HSetCommand{H: h}

		cmd.Execute("key1", "field1", "value1")
		cmd.Execute("key2", "field2", "value2")

		count := 0
		h.data.Range(func(_ string, _ *xsync.Map[string, string]) bool {
			count++
			return true
		})

		assert.Equal(t, 2, count)
	})
}

//
// HGET TESTS
//

func TestXHGetCommand(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		args      []string
		setup     func(*Hash)
		wantErr   bool
		wantValue interface{}
		wantNil   bool
	}{
		{
			name: "existing field",
			args: []string{"mykey", "field1"},
			setup: func(h *Hash) {
				cmd := &HSetCommand{H: h}
				cmd.Execute("mykey", "field1", "value1")
			},
			wantValue: "value1",
		},
		{
			name: "non-existent field",
			args: []string{"mykey", "field2"},
			setup: func(h *Hash) {
				cmd := &HSetCommand{H: h}
				cmd.Execute("mykey", "field1", "value1")
			},
			wantNil: true,
		},
		{
			name:    "non-existent key",
			args:    []string{"ghost", "field1"},
			wantNil: true,
		},
		{
			name:    "missing args",
			args:    []string{},
			wantErr: true,
		},
		{
			name: "empty key works",
			args: []string{"", "field1"},
			setup: func(h *Hash) {
				cmd := &HSetCommand{H: h}
				cmd.Execute("", "field1", "value1")
			},
			wantValue: "value1",
		},
		{
			name: "empty field works",
			args: []string{"mykey", ""},
			setup: func(h *Hash) {
				cmd := &HSetCommand{H: h}
				cmd.Execute("mykey", "", "value1")
			},
			wantValue: "value1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			h := NewVersion()
			if tt.setup != nil {
				tt.setup(h)
			}

			cmd := &HGetCommand{H: h}
			res, err := cmd.Execute(tt.args...)

			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)

			if tt.wantNil {
				assert.Nil(t, res)
				return
			}

			assert.Equal(t, tt.wantValue, res)
		})
	}

	t.Run("get all fields", func(t *testing.T) {
		t.Parallel()
		h := NewVersion()
		setCmd := &HSetCommand{H: h}
		getCmd := &HGetCommand{H: h}

		setCmd.Execute("mykey", "field1", "value1")
		setCmd.Execute("mykey", "field2", "value2")
		setCmd.Execute("mykey", "field3", "value3")

		res, err := getCmd.Execute("mykey")
		require.NoError(t, err)

		slice := res.([]string)
		assert.Len(t, slice, 6)

		m := make(map[string]string)
		for i := 0; i < len(slice); i += 2 {
			m[slice[i]] = slice[i+1]
		}

		assert.Equal(t, "value1", m["field1"])
		assert.Equal(t, "value2", m["field2"])
		assert.Equal(t, "value3", m["field3"])
	})
}

//
// FUZZ TESTS
//

func FuzzXHashSet(f *testing.F) {
	f.Add("field1", "value1")
	f.Add("", "")
	f.Add("a", "b")

	f.Fuzz(func(t *testing.T, field, value string) {
		h := NewVersion()
		root, _ := h.data.LoadOrCompute("root", func() (*xsync.Map[string, string], bool) {
			return xsync.NewMap[string, string](), false
		})

		root.Store(field, value)

		v, ok := root.Load(field)
		if ok {
			assert.Equal(t, value, v)
		}
	})
}

func FuzzXHSetCommand(f *testing.F) {
	f.Add("key1", "field1", "value1")
	f.Add("", "", "")
	f.Add("k", "f", "v")

	f.Fuzz(func(t *testing.T, key, field, value string) {
		h := NewVersion()
		setCmd := &HSetCommand{H: h}
		getCmd := &HGetCommand{H: h}

		_, err := setCmd.Execute(key, field, value)
		if err != nil {
			return
		}

		res, err := getCmd.Execute(key, field)
		if err != nil {
			return
		}

		if res != nil {
			assert.Equal(t, value, res)
		}
	})
}
