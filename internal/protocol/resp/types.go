package resp

import "fmt"

type Type int

const (
	SimpleString Type = iota
	Error
	Integer
	BulkString
	Array
)

type Value struct {
	Type Type

	Str   string
	Int   int64
	Bulk  []byte
	Array []Value
}

type ProtoError struct {
	Msg string
}

func (e ProtoError) Error() string {
	return fmt.Sprintf("resp protocol error: %s", e.Msg)
}
