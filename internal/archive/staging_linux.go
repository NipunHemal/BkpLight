package archive

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"

	"bkplight.local/bkplight/internal/safeerr"
	"golang.org/x/sys/unix"
)

type Options struct {
	Directory    string
	MaxBytes     int64
	MinFreeBytes int64
}

type ArtifactFile struct {
	RelativePath     string
	CiphertextSize   int64
	CiphertextSHA256 string
}

type Stager struct {
	mu            sync.Mutex
	dir           *os.File
	opts          Options
	used          int64
	reserved      int64
	active        map[string]bool
	syncDirectory func() error
	closed        bool
}

type Session struct {
	mu       sync.Mutex
	stager   *Stager
	ctx      context.Context
	id       string
	file     *os.File
	encrypt  io.WriteCloser
	written  int64
	reserve  int64
	renamed  bool
	finished bool
}

func stageError(code safeerr.Code) error { return &safeerr.Error{Code: code, Field: "staging"} }

// OpenStager owns a private directory by a retained no-follow descriptor.
func OpenStager(o Options) (*Stager, error) {
	if !filepath.IsAbs(o.Directory) || filepath.Clean(o.Directory) != o.Directory || o.Directory == "/" || o.MaxBytes <= 0 || o.MinFreeBytes < 0 {
		return nil, stageError(safeerr.Invalid)
	}
	dir, err := acquireDirectory(o.Directory)
	if err != nil {
		return nil, err
	}
	s := &Stager{dir: dir, opts: o, active: make(map[string]bool), syncDirectory: dir.Sync}
	if err := s.scan(); err != nil {
		_ = dir.Close()
		return nil, err
	}
	return s, nil
}

func acquireDirectory(path string) (*os.File, error) {
	return acquireDirectoryAt(path, nil)
}

// acquireDirectoryAt retains each ancestor descriptor while advancing. The
// optional step hook lets tests substitute an ancestor at the exact boundary.
func acquireDirectoryAt(path string, afterStep func(string)) (*os.File, error) {
	parent, err := os.Open("/")
	if err != nil {
		return nil, stageError(safeerr.Unavailable)
	}
	dir := parent
	for _, part := range strings.Split(strings.TrimPrefix(path, "/"), "/") {
		if part == "" || part == "." || part == ".." {
			_ = dir.Close()
			return nil, stageError(safeerr.Invalid)
		}
		flags := unix.O_RDONLY | unix.O_DIRECTORY | unix.O_NOFOLLOW | unix.O_CLOEXEC
		fd, e := unix.Openat(int(dir.Fd()), part, flags, 0)
		if errors.Is(e, unix.ENOENT) {
			if makeErr := unix.Mkdirat(int(dir.Fd()), part, 0700); makeErr == nil || errors.Is(makeErr, unix.EEXIST) {
				fd, e = unix.Openat(int(dir.Fd()), part, flags, 0)
			}
		}
		if e != nil {
			_ = dir.Close()
			return nil, stageError(safeerr.Unavailable)
		}
		next := os.NewFile(uintptr(fd), part)
		_ = dir.Close()
		dir = next
		if afterStep != nil {
			afterStep(part)
		}
	}
	info, err := dir.Stat()
	if err != nil || !info.IsDir() || info.Mode().Perm()&0077 != 0 {
		_ = dir.Close()
		return nil, stageError(safeerr.Denied)
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || stat.Uid != uint32(os.Geteuid()) {
		_ = dir.Close()
		return nil, stageError(safeerr.Denied)
	}
	current, err := os.Lstat(path)
	if err != nil || !os.SameFile(info, current) {
		_ = dir.Close()
		return nil, stageError(safeerr.Unavailable)
	}
	return dir, nil
}

func validArtifactID(id string) bool {
	if len(id) != 32 {
		return false
	}
	for _, c := range id {
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f') {
			return false
		}
	}
	return true
}

func (s *Stager) scan() error {
	names, err := s.dir.Readdirnames(-1)
	if err != nil {
		return stageError(safeerr.Unavailable)
	}
	for _, name := range names {
		base, suffix, ok := strings.Cut(name, ".")
		if !ok || !validArtifactID(base) || (suffix != "age" && suffix != "partial" && suffix != "orphan") {
			return stageError(safeerr.Denied)
		}
		var stat unix.Stat_t
		if unix.Fstatat(int(s.dir.Fd()), name, &stat, unix.AT_SYMLINK_NOFOLLOW) != nil || stat.Mode&unix.S_IFMT != unix.S_IFREG || stat.Mode&0077 != 0 || stat.Nlink != 1 || stat.Uid != uint32(os.Geteuid()) || stat.Size < 0 {
			return stageError(safeerr.Denied)
		}
		if stat.Size > s.opts.MaxBytes-s.used {
			return stageError(safeerr.Busy)
		}
		s.used += stat.Size
	}
	return nil
}

func (s *Stager) freeAvailable(additional int64) bool {
	var stat unix.Statfs_t
	if unix.Fstatfs(int(s.dir.Fd()), &stat) != nil {
		return false
	}
	// Division avoids uint64 overflow from forged or unusually large statfs data.
	if stat.Bavail > uint64(^uint64(0))/uint64(stat.Bsize) {
		return true
	}
	free := stat.Bavail * uint64(stat.Bsize)
	return free >= uint64(s.opts.MinFreeBytes) && free-uint64(s.opts.MinFreeBytes) >= uint64(additional)
}

func (s *Stager) Begin(ctx context.Context, id string, recipients []string, reserveBytes int64) (*Session, error) {
	if ctx == nil || !validArtifactID(id) || reserveBytes < 0 {
		return nil, stageError(safeerr.Invalid)
	}
	if ctx.Err() != nil {
		return nil, stageError(safeerr.Unavailable)
	}
	session, err := s.beginFile(id, reserveBytes)
	if err != nil {
		return nil, err
	}
	session.ctx = ctx
	// age writes its header during construction, so the budget mutex must not
	// be held here: the ciphertext writer acquires it for every actual write.
	w, err := NewEncryptWriter(&budgetWriter{session: session}, recipients)
	if err != nil {
		_ = session.Abort()
		return nil, err
	}
	session.encrypt = w
	return session, nil
}

func (s *Stager) beginFile(id string, reserveBytes int64) (*Session, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed || s.active[id] {
		return nil, stageError(safeerr.Conflict)
	}
	if reserveBytes > s.opts.MaxBytes-s.used-s.reserved || !s.freeAvailable(reserveBytes) {
		return nil, stageError(safeerr.Busy)
	}
	var existing unix.Stat_t
	if err := unix.Fstatat(int(s.dir.Fd()), id+".age", &existing, unix.AT_SYMLINK_NOFOLLOW); err == nil {
		return nil, stageError(safeerr.Conflict)
	} else if !errors.Is(err, unix.ENOENT) {
		return nil, stageError(safeerr.Unavailable)
	}
	fd, err := unix.Openat(int(s.dir.Fd()), id+".partial", unix.O_WRONLY|unix.O_CREAT|unix.O_EXCL|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0600)
	if err != nil {
		return nil, stageError(safeerr.Unavailable)
	}
	file := os.NewFile(uintptr(fd), id+".partial")
	session := &Session{stager: s, id: id, file: file, reserve: reserveBytes}
	s.active[id] = true
	s.reserved += reserveBytes
	return session, nil
}

type budgetWriter struct{ session *Session }

func (b *budgetWriter) Write(p []byte) (int, error) {
	session := b.session
	if session.ctx.Err() != nil {
		return 0, stageError(safeerr.Unavailable)
	}
	s := session.stager
	s.mu.Lock()
	defer s.mu.Unlock()
	additional := int64(len(p)) - session.reserve
	if additional < 0 {
		additional = 0
	}
	if s.closed || additional > s.opts.MaxBytes-s.used-s.reserved || !s.freeAvailable(int64(len(p))) {
		return 0, stageError(safeerr.Busy)
	}
	n, err := session.file.Write(p)
	consumed := min(int64(n), session.reserve)
	session.reserve -= consumed
	s.reserved -= consumed
	session.written += int64(n)
	s.used += int64(n)
	if err != nil {
		return n, stageError(safeerr.Unavailable)
	}
	return n, nil
}

func (session *Session) Write(p []byte) (int, error) {
	session.mu.Lock()
	defer session.mu.Unlock()
	if session.finished {
		return 0, stageError(safeerr.Conflict)
	}
	if session.ctx.Err() != nil {
		return 0, stageError(safeerr.Unavailable)
	}
	return session.encrypt.Write(p)
}

func (session *Session) Finalize() (ArtifactFile, error) {
	session.mu.Lock()
	defer session.mu.Unlock()
	if session.finished {
		return ArtifactFile{}, stageError(safeerr.Conflict)
	}
	if session.ctx.Err() != nil {
		return ArtifactFile{}, stageError(safeerr.Unavailable)
	}
	if err := session.encrypt.Close(); err != nil {
		return ArtifactFile{}, stageError(safeerr.Unavailable)
	}
	syncErr := session.file.Sync()
	closeErr := session.file.Close()
	if syncErr != nil || closeErr != nil {
		return ArtifactFile{}, stageError(safeerr.Unavailable)
	}
	s := session.stager
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := unix.Renameat2(int(s.dir.Fd()), session.id+".partial", int(s.dir.Fd()), session.id+".age", unix.RENAME_NOREPLACE); err != nil {
		return ArtifactFile{}, stageError(safeerr.Unavailable)
	}
	session.renamed = true
	if s.syncDirectory() != nil {
		return ArtifactFile{}, stageError(safeerr.Unavailable)
	}
	fd, err := unix.Openat(int(s.dir.Fd()), session.id+".age", unix.O_RDONLY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
	if err != nil {
		return ArtifactFile{}, stageError(safeerr.Unavailable)
	}
	file := os.NewFile(uintptr(fd), session.id+".age")
	hash := sha256.New()
	_, copyErr := io.Copy(hash, file)
	closeErr = file.Close()
	if copyErr != nil || closeErr != nil {
		return ArtifactFile{}, stageError(safeerr.Unavailable)
	}
	s.reserved -= session.reserve
	delete(s.active, session.id)
	session.finished = true
	return ArtifactFile{RelativePath: session.id + ".age", CiphertextSize: session.written, CiphertextSHA256: hex.EncodeToString(hash.Sum(nil))}, nil
}

func (session *Session) Abort() error {
	session.mu.Lock()
	defer session.mu.Unlock()
	return session.abortLocked()
}

func (session *Session) abortLocked() error {
	if session.finished {
		return nil
	}
	s := session.stager
	s.mu.Lock()
	defer s.mu.Unlock()
	closeErr := session.file.Close()
	if session.renamed && errors.Is(closeErr, os.ErrClosed) {
		closeErr = nil
	}
	var removeErr error
	if !session.renamed {
		removeErr = unix.Unlinkat(int(s.dir.Fd()), session.id+".partial", 0)
	}
	s.reserved -= session.reserve
	if !session.renamed {
		s.used -= session.written
	}
	delete(s.active, session.id)
	session.finished = true
	if closeErr != nil || removeErr != nil || s.syncDirectory() != nil {
		return stageError(safeerr.Unavailable)
	}
	return nil
}

func (s *Stager) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.active) != 0 {
		return stageError(safeerr.Busy)
	}
	if s.closed {
		return nil
	}
	s.closed = true
	if s.dir.Close() != nil {
		return stageError(safeerr.Unavailable)
	}
	return nil
}
