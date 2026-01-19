package aof

import "time"

// Appends mutating operations to durable storage
type Writer interface {
	Append(cmd string, args []string) error
	Sync() error
	Close() error
}

// Load operations from durable storage and apply them to a store
type Replayer interface {
	Replay(apply func(cmd string, args []string) error) error
}

// disabled AOF implementation
type Noop struct{}

func NewNoop() *Noop { return &Noop{} }

func (n *Noop) Append(cmd string, args []string) error { return nil }
func (n *Noop) Sync() error                            { return nil }
func (n *Noop) Close() error                           { return nil }

var _ = time.Second
