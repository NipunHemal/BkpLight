package state

import (
	"bkplight.local/bkplight/internal/safeerr"
	"os"
	"path/filepath"
	"syscall"
)

func unavailable() error { return &safeerr.Error{Code: safeerr.Unavailable} }

// acquireDirectory walks without following symlinks and retains the directory
// descriptor so all state names are interpreted relative to the acquired owner.
func acquireDirectory(path string) (*os.File, error) {
	if !filepath.IsAbs(path) || filepath.Clean(path) != path {
		return nil, unavailable()
	}
	for p := path; ; p = filepath.Dir(p) {
		info, err := os.Lstat(p)
		if err != nil && !os.IsNotExist(err) {
			return nil, unavailable()
		}
		if err == nil && !info.IsDir() {
			return nil, unavailable()
		}
		if p == filepath.Dir(p) {
			break
		}
	}
	if err := os.MkdirAll(path, 0700); err != nil {
		return nil, unavailable()
	}
	before, err := os.Lstat(path)
	if err != nil {
		return nil, unavailable()
	}
	fd, err := syscall.Open(path, syscall.O_RDONLY|syscall.O_DIRECTORY|syscall.O_NOFOLLOW|syscall.O_CLOEXEC, 0)
	if err != nil {
		return nil, unavailable()
	}
	dir := os.NewFile(uintptr(fd), path)
	info, err := dir.Stat()
	if err != nil || !os.SameFile(before, info) || !privateOwned(info, true) {
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
