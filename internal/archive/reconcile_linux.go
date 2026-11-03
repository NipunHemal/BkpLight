package archive

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"strings"

	"bkplight.local/bkplight/internal/safeerr"
	"golang.org/x/sys/unix"
)

func validSHA256(value string) bool {
	if len(value) != 64 {
		return false
	}
	for _, c := range value {
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f') {
			return false
		}
	}
	return true
}

// ReconcilePartials discards only incomplete, owned partial files after the
// catalog has marked interrupted writers. It never removes finalized files.
func (s *Stager) ReconcilePartials() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed || len(s.active) != 0 {
		return stageError(safeerr.Busy)
	}
	fd, err := unix.Openat(int(s.dir.Fd()), ".", unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
	if err != nil {
		return stageError(safeerr.Unavailable)
	}
	listing := os.NewFile(uintptr(fd), "staging")
	names, readErr := listing.Readdirnames(-1)
	closeErr := listing.Close()
	if readErr != nil || closeErr != nil {
		return stageError(safeerr.Unavailable)
	}
	for _, name := range names {
		id, suffix, ok := strings.Cut(name, ".")
		if !ok || suffix != "partial" || !validArtifactID(id) {
			continue
		}
		var stat unix.Stat_t
		if unix.Fstatat(int(s.dir.Fd()), name, &stat, unix.AT_SYMLINK_NOFOLLOW) != nil || stat.Mode&unix.S_IFMT != unix.S_IFREG || stat.Mode&0077 != 0 || stat.Nlink != 1 || stat.Uid != uint32(os.Geteuid()) || stat.Size < 0 || stat.Size > s.used {
			return stageError(safeerr.Unavailable)
		}
		if unix.Unlinkat(int(s.dir.Fd()), name, 0) != nil {
			return stageError(safeerr.Unavailable)
		}
		s.used -= stat.Size
	}
	if s.dir.Sync() != nil {
		return stageError(safeerr.Unavailable)
	}
	return nil
}

// QuarantineUncataloged prevents a final file that was renamed before a
// catalog commit from appearing eligible. Encrypted bytes remain for recovery.
// The caller supplies IDs for both cataloged archives and their manifests.
func (s *Stager) QuarantineUncataloged(known map[string]bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed || len(s.active) != 0 {
		return stageError(safeerr.Busy)
	}
	fd, err := unix.Openat(int(s.dir.Fd()), ".", unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
	if err != nil {
		return stageError(safeerr.Unavailable)
	}
	listing := os.NewFile(uintptr(fd), "staging")
	names, readErr := listing.Readdirnames(-1)
	closeErr := listing.Close()
	if readErr != nil || closeErr != nil {
		return stageError(safeerr.Unavailable)
	}
	for _, name := range names {
		id, suffix, ok := strings.Cut(name, ".")
		if !ok || suffix != "age" || !validArtifactID(id) || known[id] {
			continue
		}
		var stat unix.Stat_t
		if unix.Fstatat(int(s.dir.Fd()), name, &stat, unix.AT_SYMLINK_NOFOLLOW) != nil || stat.Mode&unix.S_IFMT != unix.S_IFREG || stat.Mode&0077 != 0 || stat.Nlink != 1 || stat.Uid != uint32(os.Geteuid()) {
			return stageError(safeerr.Unavailable)
		}
		if unix.Renameat2(int(s.dir.Fd()), name, int(s.dir.Fd()), id+".orphan", unix.RENAME_NOREPLACE) != nil {
			return stageError(safeerr.Unavailable)
		}
	}
	if s.dir.Sync() != nil {
		return stageError(safeerr.Unavailable)
	}
	return nil
}

// Verify checks a completed ciphertext against its catalog receipt without
// following links or trusting a path outside the acquired staging directory.
func (s *Stager) Verify(name string, size int64, digest string) error {
	id, suffix, ok := strings.Cut(name, ".")
	if !ok || !validArtifactID(id) || suffix != "age" || size <= 0 || !validSHA256(digest) {
		return stageError(safeerr.Invalid)
	}
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return stageError(safeerr.Unavailable)
	}
	fd, err := unix.Openat(int(s.dir.Fd()), name, unix.O_RDONLY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
	s.mu.Unlock()
	if err != nil {
		return stageError(safeerr.Unavailable)
	}
	file := os.NewFile(uintptr(fd), name)
	defer file.Close()
	var stat unix.Stat_t
	if unix.Fstat(fd, &stat) != nil || stat.Mode&unix.S_IFMT != unix.S_IFREG || stat.Mode&0077 != 0 || stat.Nlink != 1 || stat.Uid != uint32(os.Geteuid()) || stat.Size != size {
		return stageError(safeerr.Unavailable)
	}
	hash := sha256.New()
	n, err := io.Copy(hash, file)
	if err != nil || n != size || hex.EncodeToString(hash.Sum(nil)) != digest {
		return stageError(safeerr.Unavailable)
	}
	return nil
}
