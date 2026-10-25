package observability

import (
	"fmt"
	"math"
	"os"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"unsafe"
)

const activeName = "bkplight.log"

func privateDirectory(info os.FileInfo) bool {
	st, ok := info.Sys().(*syscall.Stat_t)
	return ok && info.IsDir() && info.Mode().Perm()&0022 == 0 && st.Uid == uint32(os.Geteuid())
}
func owned(info os.FileInfo) bool {
	st, ok := info.Sys().(*syscall.Stat_t)
	return ok && info.Mode().IsRegular() && info.Mode().Perm() == 0600 && st.Uid == uint32(os.Geteuid()) && st.Nlink == 1
}

func archiveID(name string) (uint64, bool) {
	if len(name) != len("bkplight.")+20+len(".log") || !strings.HasPrefix(name, "bkplight.") || !strings.HasSuffix(name, ".log") {
		return 0, false
	}
	digits := strings.TrimSuffix(strings.TrimPrefix(name, "bkplight."), ".log")
	for _, c := range digits {
		if c < '0' || c > '9' {
			return 0, false
		}
	}
	n, e := strconv.ParseUint(digits, 10, 64)
	return n, e == nil
}

func (l *Logger) entries() ([]os.DirEntry, error) {
	f, e := l.root.Open(".")
	if e != nil {
		return nil, e
	}
	defer f.Close()
	return f.ReadDir(-1)
}

// Count every reserved suffix, including foreign entries, to avoid overwriting.
func (l *Logger) scan() error {
	entries, e := l.entries()
	if e != nil {
		return e
	}
	for _, entry := range entries {
		if n, ok := archiveID(entry.Name()); ok && n >= l.next {
			if n == math.MaxUint64 {
				return unavailable()
			}
			l.next = n + 1
		}
	}
	if l.next == 0 {
		l.next = 1
	}
	return nil
}

func (l *Logger) openActive() error {
	before, e := l.root.Lstat(activeName)
	if e != nil && !os.IsNotExist(e) {
		return e
	}
	if e == nil && !owned(before) {
		return unavailable()
	}
	// O_NONBLOCK also prevents a raced FIFO from blocking startup.
	f, e := l.root.OpenFile(activeName, os.O_CREATE|os.O_APPEND|os.O_RDWR|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0600)
	if e != nil {
		return e
	}
	info, e := f.Stat()
	if e != nil || !owned(info) || (before != nil && !os.SameFile(before, info)) {
		_ = f.Close()
		return unavailable()
	}
	if info.Size() > l.opts.MaxBytes {
		_ = f.Close()
		return unavailable()
	}
	if info.Size() > 0 {
		var tail [1]byte
		if _, e = f.ReadAt(tail[:], info.Size()-1); e != nil || tail[0] != '\n' {
			_ = f.Close()
			return unavailable()
		}
	}
	l.writer = f
	l.activeInfo = info
	l.size = info.Size()
	return nil
}

func (l *Logger) rotate() error {
	if e := l.writer.Close(); e != nil {
		return e
	}
	info, e := l.root.Lstat(activeName)
	if e != nil || !owned(info) || !os.SameFile(l.activeInfo, info) {
		return unavailable()
	}
	if e = l.scan(); e != nil {
		return e
	}
	if l.next == math.MaxUint64 {
		return unavailable()
	}
	name := fmt.Sprintf("bkplight.%020d.log", l.next)
	l.next++
	if e = renameArchive(l.root, name); e != nil {
		return e
	}
	now := l.opts.Now()
	if e = l.root.Chtimes(name, now, now); e != nil {
		return e
	}
	return l.openActive()
}

// renameArchive is one atomic Linux operation; it never replaces a destination.
// Both names are simple owned names relative to the same anchored directory fd.
// The frozen syscall package lacks Renameat2 on amd64. Numbers below come from
// Linux asm/unistd_64.h (amd64) and asm-generic/unistd.h (arm64); flags: linux/fs.h.
// Unsupported architectures/kernels/filesystems fail closed, with no fallback.
func renameArchive(root logRoot, name string) error {
	if _, ok := archiveID(name); !ok {
		return unavailable()
	}
	var number uintptr
	switch runtime.GOARCH {
	case "amd64":
		number = 316
	case "arm64":
		number = 276
	default:
		return syscall.ENOSYS
	}
	directory, err := root.Open(".")
	if err != nil {
		return err
	}
	defer directory.Close()
	oldName, err := syscall.BytePtrFromString(activeName)
	if err != nil {
		return err
	}
	newName, err := syscall.BytePtrFromString(name)
	if err != nil {
		return err
	}
	const renameNoReplace = 1
	fd := directory.Fd()
	_, _, errno := syscall.Syscall6(number, fd, uintptr(unsafe.Pointer(oldName)), fd, uintptr(unsafe.Pointer(newName)), renameNoReplace, 0)
	runtime.KeepAlive(oldName)
	runtime.KeepAlive(newName)
	if errno != 0 {
		return errno
	}
	return nil
}

type archive struct {
	name string
	id   uint64
	info os.FileInfo
}

func (l *Logger) prune() error {
	entries, e := l.entries()
	if e != nil {
		return e
	}
	archives := make([]archive, 0, len(entries))
	cutoff := l.opts.Now().Add(-l.opts.MaxAge)
	for _, entry := range entries {
		id, ok := archiveID(entry.Name())
		if !ok {
			continue
		}
		info, e := l.root.Lstat(entry.Name())
		if e != nil {
			return e
		}
		if !owned(info) {
			continue
		}
		if info.ModTime().Before(cutoff) {
			if e = l.removeArchive(archive{entry.Name(), id, info}); e != nil {
				return e
			}
			continue
		}
		archives = append(archives, archive{entry.Name(), id, info})
	}
	sort.Slice(archives, func(i, j int) bool { return archives[i].id < archives[j].id })
	excess := len(archives) - (l.opts.MaxFiles - 1)
	for i := 0; i < excess; i++ {
		if e = l.removeArchive(archives[i]); e != nil {
			return e
		}
	}
	return nil
}

func (l *Logger) removeArchive(a archive) error {
	info, e := l.root.Lstat(a.name)
	if e != nil {
		return e
	}
	if !owned(info) || !os.SameFile(a.info, info) {
		return unavailable()
	}
	return l.root.Remove(a.name)
}
