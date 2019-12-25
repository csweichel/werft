package store_test

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/32leaves/werft/pkg/store"
)

func TestContinuousWriteReading(t *testing.T) {
	base, err := ioutil.TempDir(os.TempDir(), "tcwr")
	if err != nil {
		t.Errorf("cannot create test folder: %v", err)
	}

	s, err := store.NewFileLogStore(base)
	if err != nil {
		t.Errorf("cannot create test store: %v", err)
	}

	w, err := s.Open("foo")
	if err != nil {
		t.Errorf("cannot place log: %v", err)
	}
	r, err := s.Read("foo")
	if err != nil {
		t.Errorf("cannot read log: %v", err)
	}

	var msg = `hello world
	this is a test
	we're just writing stuff
	line by line`

	var wg sync.WaitGroup
	wg.Add(2)
	sync := make(chan struct{})
	go func() {
		defer wg.Done()

		for _, l := range strings.Split(msg, "\n") {
			n, err := w.Write([]byte(l))
			if err != nil {
				t.Errorf("write error: %v", err)
				return
			}
			if n != len(l) {
				t.Errorf("write error: %v", io.ErrShortWrite)
				return
			}
			sync <- struct{}{}
		}
		w.Close()
		close(sync)
	}()

	read := bytes.NewBuffer(nil)
	go func() {
		defer wg.Done()

		err := io.Copy(read, r)
		if err != nil {
			t.Errorf("cannot read log: %+v", err)
			return
		}
	}()
	wg.Wait()
}
