package store

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// FileLogStore is a file backed log store
type FileLogStore struct {
	Base string

	mu    sync.Mutex
	files map[string]*file
}

type file struct {
	closed bool
	fn     string
	fp     *os.File
	cond   *sync.Cond
}

// NewFileLogStore creates a new file backed log store
func NewFileLogStore(base string) (*FileLogStore, error) {
	f := &FileLogStore{
		Base:  base,
		files: make(map[string]*file),
	}
	err := f.init()
	if err != nil {
		return nil, err
	}

	return f, nil
}

func (fs *FileLogStore) init() error {
	logs, err := ioutil.ReadDir(fs.Base)
	if err != nil {
		return err
	}

	for _, l := range logs {
		f := &file{
			closed: true,
			fn:     l.Name(),
			cond:   sync.NewCond(&sync.Mutex{}),
		}
		f.Close()
		fs.files[strings.TrimSuffix(l.Name(), ".log")] = f
	}

	return nil
}

// Place places a logfile in this store.
func (fs *FileLogStore) Place(ctx context.Context, id string) (io.WriteCloser, error) {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	if f, exists := fs.files[id]; exists {
		return f, nil
	}

	fn := fmt.Sprintf("%s.log", id)
	fp, err := os.OpenFile(filepath.Join(fs.Base, fn), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return nil, err
	}
	f := &file{
		fn:   fn,
		fp:   fp,
		cond: sync.NewCond(&sync.Mutex{}),
	}
	fs.files[id] = f
	return f, nil
}

func (f *file) Write(b []byte) (n int, err error) {
	f.cond.L.Lock()
	defer f.cond.L.Unlock()

	n, err = f.fp.Write(b)
	if n > 0 {
		f.cond.Broadcast()
	}
	return n, err
}

func (f *file) Close() error {
	f.cond.L.Lock()
	defer f.cond.L.Unlock()

	if f.closed {
		return io.ErrClosedPipe
	}

	f.closed = true
	err := f.fp.Close()
	if err != nil {
		return err
	}
	f.cond.Broadcast()

	return nil
}

func (f *file) Closed() bool {
	f.cond.L.Lock()
	defer f.cond.L.Unlock()

	return f.closed
}

// Read retrieves a log file from this store.
func (fs *FileLogStore) Read(ctx context.Context, id string) (io.ReadCloser, error) {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	f, ok := fs.files[id]
	if !ok {
		return nil, ErrNotFound
	}

	fp, err := os.OpenFile(filepath.Join(fs.Base, f.fn), os.O_RDONLY, 0644)
	if err != nil {
		return nil, err
	}

	return &fileReader{f: f, fp: fp}, nil
}

type fileReader struct {
	f  *file
	fp io.ReadCloser
}

func (fr *fileReader) Read(p []byte) (n int, err error) {
	n, err = fr.fp.Read(p)
	if err != io.EOF {
		return
	}

	// we're done reading the file for now
	// check if we're actually done
	if fr.f.Closed() {
		err = io.EOF
		return
	}

	// if we did read something, return that
	if n > 0 {
		return n, nil
	}

	// we didn't read anything, so let's wait for more data to be written
	fr.f.cond.L.Lock()
	fr.f.cond.Wait()
	fr.f.cond.L.Unlock()
	n, err = fr.fp.Read(p)
	return
}

func (fr *fileReader) Close() error {
	return fr.fp.Close()
}
