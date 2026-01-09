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

// helper to standardize flush policies
type FsyncPolicy int

const (
	FsyncNever FsyncPolicy = iota
	FsyncAlways
	FsyncEverySecond
)

func (p FsyncPolicy) String() string {
	switch p {
	case FsyncAlways:
		return "always"
	case FsyncEverySecond:
		return "everysec"
	default:
		return "never"
	}
}

// Small helper for parsing config later
func ParseFsyncPolicy(s string) FsyncPolicy {
	switch s {
	case "always":
		return FsyncAlways
	case "everysec":
		return FsyncEverySecond
	default:
		return FsyncNever
	}
}

var _ = time.Second
