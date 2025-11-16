package resp

// RESP type markers
const (
	SimpleStringPrefix = '+'
	ErrorPrefix        = '-'
	IntegerPrefix      = ':'
	BulkStringPrefix   = '$'
	ArrayPrefix        = '*'
	CR                 = '\r'
	LF                 = '\n'
)

// RespType represents the kind of RESP value.
type RespType uint8

const (
	TypeSimpleString RespType = iota
	TypeError
	TypeInteger
	TypeBulkString
	TypeArray
)

// RespValue wraps a RESP value with explicit type info.
type RespValue struct {
	Type RespType
	Val  any
}

func (v RespValue) AsString() (string, bool) {
	s, ok := v.Val.(string)
	return s, ok
}

func (v RespValue) AsError() (error, bool) {
	e, ok := v.Val.(error)
	return e, ok
}

func (v RespValue) AsInt() (int64, bool) {
	n, ok := v.Val.(int64)
	return n, ok
}

func (v RespValue) AsBulkString() ([]byte, bool) {
	bs, ok := v.Val.([]byte)
	return bs, ok
}

func (v RespValue) AsArray() ([]*RespValue, bool) {
	arr, ok := v.Val.([]*RespValue)
	return arr, ok
}
