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
	"time"

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
	go func() {
		defer wg.Done()

		lines := strings.Split(msg, "\n")
		for i, l := range lines {
			if i < len(lines)-1 {
				l += "\n"
			}

			n, err := w.Write([]byte(l))
			if err != nil {
				panic(fmt.Errorf("write error: %v", err))
			}
			if n != len(l) {
				panic(fmt.Errorf("write error: %v", io.ErrShortWrite))
			}
			time.Sleep(10 * time.Millisecond)
		}
		w.Close()
	}()

	rbuf := bytes.NewBuffer(nil)
	go func() {
		defer wg.Done()

		_, err := io.Copy(rbuf, r)
		if err != nil {
			t.Errorf("cannot read log: %+v", err)
			return
		}
	}()

	go func() {
		time.Sleep(5 * time.Second)
		panic("timeout")
	}()
	wg.Wait()

	actual := rbuf.Bytes()
	expected := []byte(msg)
	if !bytes.Equal(actual, expected) {
		for i, c := range actual {
			if i >= len(expected) {
				t.Errorf("read more than was written at byte %d: %v", i, c)
				continue
			}
			if c != expected[i] {
				t.Errorf("read difference at byte %d: %v !== %v", i, c, expected[i])
			}
		}
		t.Errorf("did not read message back, but: %s", string(actual))
	}
}
