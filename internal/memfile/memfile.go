// Package memfile handler basic read and write to a kdfs file
package memfile

import (
	"syscall"
)

type MemFile struct {
	buf []byte
}

func New(size int) *MemFile {
	return &MemFile{
		buf: make([]byte, 0, size),
	}
}

func (f *MemFile) Len() int      { return len(f.buf) }
func (f *MemFile) Bytes() []byte { return f.buf }

func (f *MemFile) ensureCapacity(n int64) {
	if n <= int64(len(f.buf)) {
		return
	}
	needed := n - int64(len(f.buf))
	f.buf = append(f.buf, make([]byte, needed)...)
}

func (f *MemFile) ReadAt(p []byte, off int64) (int, syscall.Errno) {
	if off < 0 {
		return 0, syscall.EINVAL
	}
	if off >= int64(len(f.buf)) {
		return 0, 0 // EOF
	}
	n := copy(p, f.buf[off:])
	return n, 0
}

func (f *MemFile) WriteAt(p []byte, off int64) (int, syscall.Errno) {
	if off < 0 {
		return 0, syscall.EINVAL
	}

	end := off + int64(len(p))
	if end > int64(len(f.buf)) {
		f.ensureCapacity(end)
	}

	n := copy(f.buf[off:], p)
	return n, 0
}
