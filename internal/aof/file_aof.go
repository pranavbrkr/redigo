package aof

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"

	"github.com/pranavbrkr/redigo/internal/protocol/resp"
)

type FileAOF struct {
	mu     sync.Mutex
	f      *os.File
	w      *bufio.Writer
	path   string
	closed bool
}

func Open(path string) (*FileAOF, error) {
	// ensure parent directory exists
	dir := filepath.Dir(path)
	if dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("mkdir %s: %w", dir, err)
		}
	}

	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, fmt.Errorf("open aof %s: %w", path, err)
	}

	return &FileAOF{
		f:    f,
		w:    bufio.NewWriterSize(f, 64*1024),
		path: path,
	}, nil
}

func (a *FileAOF) Append(cmd string, args []string) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.closed {
		return fmt.Errorf("aof closed")
	}

	// Encode as RESP Array of Bulk strings: [CMD, arg1, arg2, ...]
	_ = resp.WriteArrayHeader(a.w, 1+len(args))
	if err := resp.WriteBulkString(a.w, []byte(cmd)); err != nil {
		return err
	}
	for _, s := range args {
		if err := resp.WriteBulkString(a.w, []byte(s)); err != nil {
			return err
		}
	}

	return a.w.Flush()
}

func (a *FileAOF) Sync() error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.closed {
		return nil
	}

	if err := a.w.Flush(); err != nil {
		return err
	}
	return a.f.Sync()
}

func (a *FileAOF) Close() error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.closed {
		return nil
	}
	a.closed = true

	_ = a.w.Flush()
	return a.f.Close()
}

// Replay the reads the AOF file from the disk and calls apply(cmd, args) for each entry
// Expects each entry to be RESP Array of Bulk Strings
func Replay(path string, apply func(cmd string, args []string) error) error {
	f, err := os.Open(path)
	if err != nil {
		// If file does not exist, nothing to replay
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("open aof for replay %s: %w", path, err)
	}
	defer f.Close()

	r := bufio.NewReaderSize(f, 64*1024)

	for {
		v, err := resp.Decode(r)
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return fmt.Errorf("decode aof: %w", err)
		}

		cmd, args, ok := decodeAOFCommand(v)
		if !ok {
			return fmt.Errorf("invalid aof entry (expected array of bulk strings)")
		}

		if err := apply(cmd, args); err != nil {
			return fmt.Errorf("apply %s: %w", cmd, err)
		}
	}
}

func decodeAOFCommand(v resp.Value) (string, []string, bool) {
	if v.Type != resp.Array || len(v.Array) == 0 {
		return "", nil, false
	}

	first := v.Array[0]
	if first.Type != resp.BulkString || first.Bulk == nil {
		return "", nil, false
	}
	cmd := string(first.Bulk)

	args := make([]string, 0, len(v.Array)-1)
	for i := 1; i < len(v.Array); i++ {
		el := v.Array[i]
		if el.Type != resp.BulkString || el.Bulk == nil {
			return "", nil, false
		}
		args = append(args, string(el.Bulk))
	}

	return cmd, args, true
}
