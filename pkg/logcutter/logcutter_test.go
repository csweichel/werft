package logcutter_test

import (
	"bytes"
	"fmt"
	"reflect"
	"strings"
	"testing"

	v1 "github.com/32leaves/werft/pkg/api/v1"
	"github.com/32leaves/werft/pkg/logcutter"
)

func TestDefaultCutterSlice(t *testing.T) {
	tests := []struct {
		Input  string
		Events []v1.LogSliceEvent
		Error  error
	}{
		{
			`
[foobar] Hello World this is a test
[otherproc] Some other process
[foobar] More output
[foobar|EOF]
[otherproc] Cool beans
			`,
			[]v1.LogSliceEvent{
				v1.LogSliceEvent{Name: "foobar", Phase: v1.LogSlicePhase_SLICE_START},
				v1.LogSliceEvent{Name: "foobar", Phase: v1.LogSlicePhase_SLICE_CONTENT, Payload: "Hello World this is a test"},
				v1.LogSliceEvent{Name: "otherproc", Phase: v1.LogSlicePhase_SLICE_START},
				v1.LogSliceEvent{Name: "otherproc", Phase: v1.LogSlicePhase_SLICE_CONTENT, Payload: "Some other process"},
				v1.LogSliceEvent{Name: "foobar", Phase: v1.LogSlicePhase_SLICE_CONTENT, Payload: "More output"},
				v1.LogSliceEvent{Name: "foobar", Phase: v1.LogSlicePhase_SLICE_END},
				v1.LogSliceEvent{Name: "otherproc", Phase: v1.LogSlicePhase_SLICE_CONTENT, Payload: "Cool beans"},
				v1.LogSliceEvent{Name: "otherproc", Phase: v1.LogSlicePhase_SLICE_ABANDONED},
			},
			nil,
		},
		{
			`
[build|CHECKPOINT] Pushing foobar
[components/foobar:docker] c13a632cd17b: Preparing
			`,
			[]v1.LogSliceEvent{
				v1.LogSliceEvent{Name: "build", Phase: v1.LogSlicePhase_SLICE_CHECKPOINT, Payload: "Pushing foobar"},
				v1.LogSliceEvent{Name: "components/foobar:docker", Phase: v1.LogSlicePhase_SLICE_START},
				v1.LogSliceEvent{Name: "components/foobar:docker", Phase: v1.LogSlicePhase_SLICE_CONTENT, Payload: "c13a632cd17b: Preparing"},
				v1.LogSliceEvent{Name: "components/foobar:docker", Phase: v1.LogSlicePhase_SLICE_ABANDONED},
			},
			nil,
		},
	}

	for _, test := range tests {
		content := strings.TrimSpace(test.Input)
		evtchan, errchan := logcutter.DefaultCutter.Slice(bytes.NewReader([]byte(content)))

		var (
			events []v1.LogSliceEvent
			err    error
		)
	recv:
		for {
			select {
			case evt := <-evtchan:
				if evt == nil {
					break recv
				}

				events = append(events, *evt)
			case err = <-errchan:
				break recv
			}
		}

		if err != test.Error {
			t.Errorf("unexpected error: \"%s\", expected \"%s\"", err, test.Error)
		}
		if !reflect.DeepEqual(test.Events, events) {
			expevt := make([]string, len(test.Events))
			for i, evt := range test.Events {
				expevt[i] = fmt.Sprintf("\t[%s] %s: %s", evt.Name, evt.Phase.String(), evt.Payload)
			}
			actevt := make([]string, len(events))
			for i, evt := range events {
				actevt[i] = fmt.Sprintf("\t[%s] %s: %s", evt.Name, evt.Phase.String(), evt.Payload)
			}

			t.Errorf("unexpected events:\n%s\nexpected:\n%s", strings.Join(actevt, "\n"), strings.Join(expevt, "\n"))
		}
	}
}
