package store

import (
	"fmt"
	"io"
	"io/fs"
	"io/ioutil"
	"os"
	"path/filepath"
	"sync"
	"time"

	"golang.org/x/sys/unix"
)

// FileLogStore is a file backed log store
type FileLogStore struct {
	Base string

	mu    sync.Mutex
	files map[string]*file
}

var _ Logs = &FileLogStore{}

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

	return f, nil
}

// GarbageCollect removes all log files older than the given duration.
func (fs *FileLogStore) GarbageCollect(olderThan time.Duration) error {
	// clean known files first - this way we can skip those which are currently
	// open for writing.
	openForWriting := make(map[string]struct{})
	func() {
		fs.mu.Lock()
		defer fs.mu.Unlock()

		for _, f := range fs.files {
			if !f.closed {
				openForWriting[f.fn] = struct{}{}

			}
		}
	}()

	// let's look at all the files in the base path, excluding those
	// of which we know they're open for writing.
	fss, err := ioutil.ReadDir(fs.Base)
	if err != nil {
		return err
	}
	for _, f := range fss {
		if !f.Mode().IsRegular() {
			continue
		}
		if _, ok := openForWriting[f.Name()]; ok {
			continue
		}
		if time.Since(fileAge(f)) <= olderThan {
			continue
		}

		// we don't want a single file to block the GC of others
		_ = os.Remove(filepath.Join(fs.Base, f.Name()))
	}

	return nil
}

// fileAge returns the age of a file
func fileAge(stat fs.FileInfo) time.Time {
	if lstat, ok := stat.Sys().(*unix.Stat_t); ok {
		// filesystem timestamps are hard - if this is Linux, we can get
		// the creation filestamp instad of the modification timestamp.
		return time.Unix(lstat.Ctim.Unix())
	}
	return stat.ModTime()
}

// Open places a logfile in this store and opens it for writing.
func (fs *FileLogStore) Open(id string) (io.WriteCloser, error) {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	if f, exists := fs.files[id]; exists {
		if f.Closed() {
			err := f.openForWriting(fs.Base)
			if err != nil {
				return nil, err
			}
		}

		return f, nil
	}

	fn := fmt.Sprintf("%s.log", id)
	f := &file{
		closed: true,
		fn:     fn,
		fp:     nil,
		cond:   sync.NewCond(&sync.Mutex{}),
	}
	err := f.openForWriting(fs.Base)
	if err != nil {
		return nil, err
	}
	fs.files[id] = f
	return f, nil
}

// Write provides write access to a previously placed file
func (fs *FileLogStore) Write(id string) (io.Writer, error) {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	f, exists := fs.files[id]
	if !exists {
		return nil, ErrNotFound
	}

	return f, nil
}

func (f *file) openForWriting(base string) error {
	f.cond.L.Lock()
	defer f.cond.L.Unlock()

	fp, err := os.OpenFile(filepath.Join(base, f.fn), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return err
	}
	f.fp = fp
	f.closed = false

	return nil
}

func (f *file) Write(b []byte) (n int, err error) {
	f.cond.L.Lock()
	defer f.cond.L.Unlock()

	if f.closed {
		return 0, io.ErrClosedPipe
	}

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
func (fs *FileLogStore) Read(id string) (io.ReadCloser, error) {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	f, ok := fs.files[id]
	if !ok {
		fn := fmt.Sprintf("%s.log", id)
		if _, err := os.Stat(filepath.Join(fs.Base, fn)); err != nil {
			return nil, ErrNotFound
		}

		f = &file{
			closed: true,
			fn:     fn,
			fp:     nil,
			cond:   sync.NewCond(&sync.Mutex{}),
		}
		fs.files[id] = f
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
	for {
		n, err = fr.fp.Read(p)
		if err != io.EOF {
			return
		}

		// we're done reading the file for now
		// check if we're actually done
		if err == io.EOF && fr.f.Closed() {
			return n, io.EOF
		}

		// if we did read something, return that
		if n > 0 {
			return n, nil
		}

		// we didn't read anything, so let's wait for more data to be written
		fr.f.cond.L.Lock()
		fr.f.cond.Wait()
		fr.f.cond.L.Unlock()
	}
}

func (fr *fileReader) Close() error {
	return fr.fp.Close()
}
