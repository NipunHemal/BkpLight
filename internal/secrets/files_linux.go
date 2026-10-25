// Package secrets reads bounded credential bytes through trusted root handles.
package secrets

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"syscall"

	"bkplight.local/bkplight/internal/safeerr"
)

const maxSecretBytes int64 = 64 << 10

type trustedRoot struct {
	path   string
	handle *os.Root
}

// Resolver owns directory handles, never a cache of credential contents.
type Resolver struct {
	mu       sync.RWMutex
	roots    []trustedRoot
	maxBytes int64
	closed   bool
}

func denied() error { return &safeerr.Error{Code: safeerr.Denied, Field: "secrets"} }

// New opens absolute, cleaned trusted directories and caps each read at maxBytes.
func New(roots []string, maxBytes int64) (*Resolver, error) {
	if maxBytes < 1 || maxBytes > maxSecretBytes {
		return nil, denied()
	}
	r := &Resolver{maxBytes: maxBytes}
	for _, path := range roots {
		if !filepath.IsAbs(path) || filepath.Clean(path) != path {
			r.Close()
			return nil, denied()
		}
		h, err := os.OpenRoot(path)
		if err != nil {
			r.Close()
			return nil, denied()
		}
		r.roots = append(r.roots, trustedRoot{path: path, handle: h})
	}
	// Overlapping configured roots use the most specific confinement boundary.
	sort.SliceStable(r.roots, func(i, j int) bool { return len(r.roots[i].path) > len(r.roots[j].path) })
	return r, nil
}

// Read follows only symlinks confined by the containing root. Returned bytes are
// exact file bytes; the caller owns their lifetime and format interpretation.
func (r *Resolver) Read(ctx context.Context, absolutePath string) ([]byte, error) {
	if ctx.Err() != nil {
		return nil, denied()
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	if r.closed || !filepath.IsAbs(absolutePath) || filepath.Clean(absolutePath) != absolutePath {
		return nil, denied()
	}
	for _, root := range r.roots {
		rel, err := filepath.Rel(root.path, absolutePath)
		if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
			continue
		}
		f, err := root.handle.OpenFile(rel, os.O_RDONLY|syscall.O_NONBLOCK, 0)
		if err != nil {
			return nil, denied()
		}
		defer f.Close()
		info, err := f.Stat()
		if err != nil || !info.Mode().IsRegular() || info.Size() > r.maxBytes || ctx.Err() != nil {
			return nil, denied()
		}
		data, err := io.ReadAll(io.LimitReader(f, r.maxBytes+1))
		if err != nil || int64(len(data)) > r.maxBytes || ctx.Err() != nil {
			clear(data)
			return nil, denied()
		}
		return data, nil
	}
	return nil, denied()
}

// Close releases all root handles and is safe to call repeatedly.
func (r *Resolver) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed {
		return nil
	}
	r.closed = true
	var result error
	for _, root := range r.roots {
		if root.handle.Close() != nil {
			result = denied()
		}
	}
	r.roots = nil
	return result
}
