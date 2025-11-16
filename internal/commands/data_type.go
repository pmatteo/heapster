package commands

import "time"

// DataType represents the type of stored data
type DataType int

const (
	TypeHash DataType = iota
	TypeList
	TypeSet
)

// DataObject wraps all data types with metadata
type DataObject struct {
	Type         DataType
	Data         any
	CreatedAt    time.Time
	LastAccessed time.Time
}

type Feature map[string]Command

// Command defines the contract for commands.
type Command interface {
	Execute(args ...string) (any, error)
}
