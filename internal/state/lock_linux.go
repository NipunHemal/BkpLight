package state

import (
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"bkplight.local/bkplight/internal/safeerr"
)

func unavailable() error { return &safeerr.Error{Code: safeerr.Unavailable} }

// acquireDirectory walks without following symlinks and retains the directory
// descriptor so all state names are interpreted relative to the acquired owner.
func acquireDirectory(path string) (*os.File, error) {
	if !filepath.IsAbs(path) || filepath.Clean(path) != path || path == "/" {
		return nil, unavailable()
	}
	root, err := os.Open("/")
	if err != nil {
		return nil, unavailable()
	}
	defer root.Close()
	return acquireDirectoryAt(root, path[1:])
}

func acquireDirectoryAt(root *os.File, relative string) (*os.File, error) {
	if relative == "" || filepath.IsAbs(relative) {
		return nil, unavailable()
	}
	parts := strings.Split(relative, "/")
	for _, part := range parts {
		if part == "" || part == "." || part == ".." {
			return nil, unavailable()
		}
	}
	// Retained descriptors, rather than re-resolved ancestor names, are the
	// authority at every step. A concurrent symlink replacement is either
	// rejected by O_NOFOLLOW or leaves us in the already acquired directory.
	dir := root
	for _, part := range parts {
		flags := syscall.O_RDONLY | syscall.O_DIRECTORY | syscall.O_NOFOLLOW | syscall.O_CLOEXEC
		fd, err := syscall.Openat(int(dir.Fd()), part, flags, 0)
		if err == syscall.ENOENT {
			if makeErr := syscall.Mkdirat(int(dir.Fd()), part, 0700); makeErr == nil || makeErr == syscall.EEXIST {
				fd, err = syscall.Openat(int(dir.Fd()), part, flags, 0)
			}
		}
		if err != nil {
			if dir != root {
				dir.Close()
			}
			return nil, unavailable()
		}
		next := os.NewFile(uintptr(fd), filepath.Join(dir.Name(), part))
		if dir != root {
			dir.Close()
		}
		dir = next
	}
	info, err := dir.Stat()
	if err != nil || !privateOwned(info, true) {
		dir.Close()
		return nil, unavailable()
	}
	if err = dir.Chmod(0700); err != nil {
		dir.Close()
		return nil, unavailable()
	}
	return dir, nil
}

func privateOwned(info os.FileInfo, directory bool) bool {
	if info == nil {
		return false
	}
	st, ok := info.Sys().(*syscall.Stat_t)
	return ok && st.Uid == uint32(os.Geteuid()) && info.Mode().Perm()&0022 == 0 &&
		((directory && info.IsDir()) || (!directory && info.Mode().IsRegular() && st.Nlink == 1))
}

func acquireLock(dir *os.File) (*os.File, error) {
	fd, err := syscall.Openat(int(dir.Fd()), "state.lock", syscall.O_CREAT|syscall.O_RDWR|syscall.O_NOFOLLOW|syscall.O_CLOEXEC|syscall.O_NONBLOCK, 0600)
	if err != nil {
		return nil, unavailable()
	}
	lock := os.NewFile(uintptr(fd), "state.lock")
	info, err := lock.Stat()
	if err != nil || !privateOwned(info, false) {
		lock.Close()
		return nil, unavailable()
	}
	if err = lock.Chmod(0600); err != nil {
		lock.Close()
		return nil, unavailable()
	}
	if err = syscall.Flock(fd, syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		lock.Close()
		if err == syscall.EWOULDBLOCK {
			return nil, &safeerr.Error{Code: safeerr.Conflict}
		}
		return nil, unavailable()
	}
	return lock, nil
}
