package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"

	v1 "github.com/csweichel/werft/pkg/api/v1"
	"github.com/csweichel/werft/pkg/api/v1/mock"
	"github.com/golang/mock/gomock"
	"github.com/golang/protobuf/jsonpb"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-github/v35/github"
)

func TestParseCommand(t *testing.T) {
	type Expectation struct {
		Cmd  string
		Args []string
		Err  string
	}
	tests := []struct {
		Name        string
		Input       string
		Expectation Expectation
	}{
		{Name: "empty line", Input: ""},
		{Name: "ignore line", Input: "something\nsomethingelse"},
		{Name: "no command", Input: "/werft", Expectation: Expectation{Err: "missing command"}},
		{Name: "no arg", Input: "/werft foo", Expectation: Expectation{Cmd: "foo", Args: []string{}}},
		{Name: "one arg", Input: "/werft foo bar", Expectation: Expectation{Cmd: "foo", Args: []string{"bar"}}},
		{Name: "two args", Input: "/werft foo bar=baz something", Expectation: Expectation{Cmd: "foo", Args: []string{"bar=baz", "something"}}},
	}
	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			var (
				act Expectation
				err error
			)
			act.Cmd, act.Args, err = parseCommand(test.Input)
			if err != nil {
				act.Err = err.Error()
			}

			if diff := cmp.Diff(test.Expectation, act); diff != "" {
				t.Errorf("MakeGatewayInfo() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestHandleCommandRun(t *testing.T) {
	type Expectation struct {
		StartRequest *v1.StartJobRequest2 `json:"req,omitempty"`
		Msg          string               `json:"msg,omitempty"`
		Error        string               `json:"error,omitempty"`
	}
	type Fixture struct {
		JobProtection JobProtectionLevel
		Event         *github.IssueCommentEvent
		PR            *github.PullRequest
		Args          []string
	}

	fs, err := filepath.Glob("fixtures/handleCommandRun_*.json")
	if err != nil {
		t.Fatal(err)
	}
	for _, fn := range fs {
		t.Run(fn, func(t *testing.T) {
			fc, err := ioutil.ReadFile(fn)
			if err != nil {
				t.Fatalf("cannot read %s: %v", fn, err)
			}
			var fixture Fixture
			err = json.Unmarshal(fc, &fixture)
			if err != nil {
				t.Fatalf("cannot unmarshal %s: %v", fn, err)
			}

			mockCtrl := gomock.NewController(t)
			defer mockCtrl.Finish()

			var act Expectation
			client := mock.NewMockWerftServiceClient(mockCtrl)
			client.EXPECT().StartJob2(gomock.Any(), gomock.Any(), gomock.Any()).
				MinTimes(1).
				DoAndReturn(func(ctx context.Context, creq *v1.StartJobRequest2) (*v1.StartJobResponse, error) {
					act.StartRequest = creq
					return &v1.StartJobResponse{
						Status: &v1.JobStatus{
							Name: "foo",
							Metadata: &v1.JobMetadata{
								Owner: "owner",
								Repository: &v1.Repository{
									Host:     "github.com",
									Owner:    "repo-owner",
									Repo:     "repo",
									Ref:      "ref",
									Revision: "rev",
								},
								Trigger:     v1.JobTrigger_TRIGGER_MANUAL,
								JobSpecName: "foo",
							},
						},
					}, nil
				})

			plg := &githubTriggerPlugin{
				Werft: client,
				Config: &Config{
					JobProtection: fixture.JobProtection,
				},
			}
			act.Msg, err = plg.handleCommandRun(context.Background(), fixture.Event, fixture.PR, fixture.Args)
			if err != nil {
				act.Error = err.Error()
			}

			var expectation Expectation
			goldenFN := strings.TrimSuffix(fn, filepath.Ext(fn)) + ".golden"
			var e struct {
				StartRequest json.RawMessage `json:"req,omitempty"`
				Msg          string          `json:"msg,omitempty"`
				Error        string          `json:"error,omitempty"`
			}
			if fc, err := os.ReadFile(goldenFN); err == nil {
				err = json.Unmarshal(fc, &e)
				if err != nil {
					t.Fatal(err)
				}
				if len(e.StartRequest) > 0 {
					expectation.StartRequest = &v1.StartJobRequest2{}
					err = jsonpb.Unmarshal(bytes.NewReader([]byte(e.StartRequest)), expectation.StartRequest)
					if err != nil {
						t.Fatal(err)
					}
				}
				expectation.Error = e.Error
				expectation.Msg = e.Msg
			} else {
				var m jsonpb.Marshaler
				srfc, err := m.MarshalToString(act.StartRequest)
				if err != nil {
					t.Fatal(err)
				}
				e.StartRequest = []byte(srfc)
				e.Error = act.Error
				e.Msg = act.Msg
				fc, err := json.MarshalIndent(e, "", "  ")
				if err != nil {
					t.Fatal("cannot marshal expectation: %w", err)
				}
				_ = ioutil.WriteFile(goldenFN, fc, 0644)
				t.Fatalf("no golden file present %s: wrote %s", fn, goldenFN)
			}

			if diff := cmp.Diff(expectation, act); diff != "" {
				t.Errorf("handleCommandRun() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
