package logcutter

import (
	"bufio"
	"io"
	"strings"

	v1 "github.com/csweichel/werft/pkg/api/v1"
)

// Cutter splits a log stream into slices for more structured display
type Cutter interface {
	// Slice reads on the in reader line-by-line. For each line it can produce several events
	// on the events channel. Once the reader returns EOF the events and errchan are closed.
	// If anything goes wrong while reading a single error is written to errchan, but nothing is closed.
	Slice(in io.Reader) (events <-chan *v1.LogSliceEvent, errchan <-chan error)
}

const (
	// DefaultSlice is the parent slice of all unmarked content
	DefaultSlice = "default"
)

// NoCutter does not slice the content up at all
var NoCutter Cutter = noCutter{}

type noCutter struct{}

// Slice returns all log lines
func (noCutter) Slice(in io.Reader) (events <-chan *v1.LogSliceEvent, errchan <-chan error) {
	evts := make(chan *v1.LogSliceEvent)
	errc := make(chan error)
	events, errchan = evts, errc

	scanner := bufio.NewScanner(in)
	go func() {
		for scanner.Scan() {
			line := scanner.Text()
			evts <- &v1.LogSliceEvent{
				Name:    DefaultSlice,
				Type:    v1.LogSliceType_SLICE_CONTENT,
				Payload: line + "\n",
			}
		}
		if err := scanner.Err(); err != nil {
			errc <- err
		}
		close(evts)
		close(errc)
	}()

	return
}

// DefaultCutter implements the default cutting behaviour
var DefaultCutter Cutter = defaultCutter{}

type defaultCutter struct{}

// Slice cuts a log stream into pieces based on a configurable delimiter
func (defaultCutter) Slice(in io.Reader) (events <-chan *v1.LogSliceEvent, errchan <-chan error) {
	evts := make(chan *v1.LogSliceEvent)
	errc := make(chan error)
	events, errchan = evts, errc

	scanner := bufio.NewScanner(in)
	phase := DefaultSlice
	go func() {
		idx := make(map[string]struct{})
		for scanner.Scan() {
			line := scanner.Text()
			sl := strings.TrimSpace(line)

			var (
				name    string
				verb    string
				payload string
			)

			if !(strings.HasPrefix(sl, "[") && strings.Contains(sl, "]")) {
				name = phase
				payload = line
			} else {
				start := strings.IndexRune(sl, '[')
				end := strings.IndexRune(sl, ']')
				name = sl[start+1 : end]
				payload = strings.TrimPrefix(sl[end+1:], " ")

				if segs := strings.Split(name, "|"); len(segs) == 2 {
					name = segs[0]
					verb = segs[1]
				}
			}

			switch verb {
			case "DONE":
				delete(idx, name)
				evts <- &v1.LogSliceEvent{
					Name: name,
					Type: v1.LogSliceType_SLICE_DONE,
				}
				continue
			case "FAIL":
				delete(idx, name)
				evts <- &v1.LogSliceEvent{
					Name:    name,
					Payload: payload,
					Type:    v1.LogSliceType_SLICE_FAIL,
				}
				continue
			case "RESULT":
				evts <- &v1.LogSliceEvent{
					Name:    name,
					Type:    v1.LogSliceType_SLICE_RESULT,
					Payload: payload,
				}
				continue
			case "PHASE":
				evts <- &v1.LogSliceEvent{
					Name:    name,
					Type:    v1.LogSliceType_SLICE_PHASE,
					Payload: payload,
				}
				phase = name
				continue
			}

			_, exists := idx[name]
			if !exists {
				idx[name] = struct{}{}
				evts <- &v1.LogSliceEvent{
					Name: name,
					Type: v1.LogSliceType_SLICE_START,
				}
			}
			evts <- &v1.LogSliceEvent{
				Name:    name,
				Type:    v1.LogSliceType_SLICE_CONTENT,
				Payload: string([]byte(payload)),
			}
		}
		if err := scanner.Err(); err != nil {
			errc <- err
		}

		for name := range idx {
			evts <- &v1.LogSliceEvent{
				Name: name,
				Type: v1.LogSliceType_SLICE_ABANDONED,
			}
		}

		close(evts)
		close(errc)
	}()

	return
}
