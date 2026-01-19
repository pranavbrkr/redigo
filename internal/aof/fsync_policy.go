package aof

import "strings"

type FsyncPolicy int

const (
	FsyncAlways FsyncPolicy = iota
	FsyncEverySecond
	FsyncNever
)

func (p FsyncPolicy) String() string {
	switch p {
	case FsyncAlways:
		return "always"
	case FsyncEverySecond:
		return "everysec"
	case FsyncNever:
		return "never"
	default:
		return "everysec"
	}
}

// ParseFsyncPolicy maps flag values to a policy.
// Defaults to everysec for unknown values to keep it resilient.
func ParseFsyncPolicy(s string) FsyncPolicy {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "always":
		return FsyncAlways
	case "never":
		return FsyncNever
	case "everysec", "everysecond", "1s":
		return FsyncEverySecond
	default:
		return FsyncEverySecond
	}
}
