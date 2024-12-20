package filesystem

import (
	"bufio"
	"io"
	"os"
)

type Reader interface {
	ReadString(delim byte) (string, error)
}

type reader struct {
	reader *bufio.Reader
}

func (r *reader) ReadString(delim byte) (string, error) {
	return r.reader.ReadString('\n')
}

type Writer interface {
	WriteString(s string) (n int, err error)
	Flush() error
}

type writer struct {
	writer *bufio.Writer
}

func (w *writer) WriteString(s string) (n int, err error) {
	return w.writer.WriteString(s)
}

func (w *writer) Flush() error {
	return w.writer.Flush()
}

// FileSystem is an abstraction for file system operations.
type FileSystem interface {
	MkdirAll(path string, perm os.FileMode) error
	Open(name string) (*os.File, error)
	Create(name string) (*os.File, error)
	Copy(dst io.Writer, src io.Reader) (int64, error)
	TeeReader(r io.Reader, w io.Writer) io.Reader
	NewReader(file io.Reader) Reader
	NewWriter(file io.Writer) Writer
}

type osFileSystem struct{}

func NewOsFileSystem() FileSystem {
	return &osFileSystem{}
}

func (fs *osFileSystem) MkdirAll(path string, perm os.FileMode) error {
	return os.MkdirAll(path, perm)
}

func (fs *osFileSystem) Open(name string) (*os.File, error) {
	return os.Open(name)
}

func (fs *osFileSystem) Create(name string) (*os.File, error) {
	return os.Create(name)
}

func (fs *osFileSystem) Copy(dst io.Writer, src io.Reader) (int64, error) {
	return io.Copy(dst, src)
}

func (fs *osFileSystem) TeeReader(r io.Reader, w io.Writer) io.Reader {
	return io.TeeReader(r, w)
}

func (fs *osFileSystem) NewReader(file io.Reader) Reader {
	return &reader{reader: bufio.NewReader(file)}
}

func (fs *osFileSystem) NewWriter(file io.Writer) Writer {
	return &writer{writer: bufio.NewWriter(file)}
}
