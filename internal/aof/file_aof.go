package aof

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"

	"github.com/pranavbrkr/redigo/internal/protocol/resp"
	"github.com/pranavbrkr/redigo/internal/store"
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

// Replay reads AOF from disk and calls apply(cmd,args) for each entry.
// Crash-safe: ignores a truncated final entry (common after crash).
func Replay(path string, apply func(cmd string, args []string) error) error {
	f, err := os.Open(path)
	if err != nil {
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
			// clean EOF
			if errors.Is(err, io.EOF) {
				return nil
			}

			// tolerate truncated tail:
			// if decode errored but there are no more bytes available, ignore tail and succeed
			if _, peekErr := r.Peek(1); errors.Is(peekErr, io.EOF) {
				return nil
			}

			// otherwise: corruption in the middle or malformed entry
			return fmt.Errorf("decode aof: %w", err)
		}

		cmd, args, ok := decodeAOFCommand(v)
		if !ok {
			// same truncate-tail tolerance: if this happens and we are at EOF, ignore.
			if _, peekErr := r.Peek(1); errors.Is(peekErr, io.EOF) {
				return nil
			}
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

// Rewrite compacts the AOF to a minimal set of commands representing current state.
// It is synchronous and blocks concurrent appends.
func (a *FileAOF) Rewrite(snapshot []store.SnapshotEntry) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.closed {
		return fmt.Errorf("aof closed")
	}

	// 1) Flush + fsync current AOF (best effort durability before rewrite)
	_ = a.w.Flush()
	_ = a.f.Sync()

	// 2) Write new AOF to temp file
	dir := filepath.Dir(a.path)
	tmpPath := filepath.Join(dir, filepath.Base(a.path)+".tmp")

	tmp, err := os.OpenFile(tmpPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("open temp aof %s: %w", tmpPath, err)
	}

	tmpW := bufio.NewWriterSize(tmp, 64*1024)

	writeCmd := func(cmd string, args ...string) error {
		if err := resp.WriteArrayHeader(tmpW, 1+len(args)); err != nil {
			return err
		}
		if err := resp.WriteBulkString(tmpW, []byte(cmd)); err != nil {
			return err
		}
		for _, s := range args {
			if err := resp.WriteBulkString(tmpW, []byte(s)); err != nil {
				return err
			}
		}
		return nil
	}

	for _, e := range snapshot {
		// SET key value
		if err := writeCmd("SET", e.Key, string(e.Value)); err != nil {
			_ = tmp.Close()
			_ = os.Remove(tmpPath)
			return fmt.Errorf("rewrite write SET: %w", err)
		}
		if e.ExpiresAt != nil {
			// EXPIREAT key unixSeconds
			if err := writeCmd("EXPIREAT", e.Key, fmt.Sprintf("%d", *e.ExpiresAt)); err != nil {
				_ = tmp.Close()
				_ = os.Remove(tmpPath)
				return fmt.Errorf("rewrite write EXPIREAT: %w", err)
			}
		}
	}

	if err := tmpW.Flush(); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpPath)
		return fmt.Errorf("rewrite flush: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpPath)
		return fmt.Errorf("rewrite fsync: %w", err)
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("rewrite close temp: %w", err)
	}

	// 3) Replace live AOF atomically-ish (Windows-safe)
	// On Windows, renaming over an existing file fails; remove first.
	_ = a.f.Close()

	_ = os.Remove(a.path) // ignore error if doesn't exist
	if err := os.Rename(tmpPath, a.path); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("rewrite replace: %w", err)
	}

	// 4) Reopen for appends
	f, err := os.OpenFile(a.path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("rewrite reopen aof: %w", err)
	}
	a.f = f
	a.w = bufio.NewWriterSize(f, 64*1024)

	return nil
}
