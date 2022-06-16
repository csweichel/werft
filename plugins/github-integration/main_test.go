package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	v1 "github.com/csweichel/werft/pkg/api/v1"
	"github.com/csweichel/werft/pkg/api/v1/mock"
	"github.com/golang/mock/gomock"
	"github.com/golang/protobuf/jsonpb"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
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
		{Name: "no command", Input: "/werft", Expectation: Expectation{Err: "cannot parse : missing command"}},
		{Name: "no arg", Input: "/werft foo", Expectation: Expectation{Cmd: "foo", Args: []string{}}},
		{Name: "one arg", Input: "/werft foo bar", Expectation: Expectation{Cmd: "foo", Args: []string{"bar"}}},
		{Name: "two args", Input: "/werft foo bar=baz something", Expectation: Expectation{Cmd: "foo", Args: []string{"bar=baz", "something"}}},
		{Name: "with newline", Input: "/werft run foo=bar arg1\n\n:-1:   ", Expectation: Expectation{Cmd: "run", Args: []string{"foo=bar", "arg1"}}},
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
				t.Errorf("parseCommand() mismatch (-want +got):\n%s", diff)
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
					sort.Slice(creq.Metadata.Annotations, func(i, j int) bool {
						return creq.Metadata.Annotations[i].Key < creq.Metadata.Annotations[j].Key
					})
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
			_, args, err := parseCommand(fixture.Event.GetComment().GetBody())
			if err != nil {
				t.Fatal(err)
			}
			act.Msg, err = plg.handleCommandRun(context.Background(), fixture.Event, fixture.PR, args)
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

func TestProcessPushEvent(t *testing.T) {
	type Fixture struct {
		Event *github.PushEvent
	}

	fs, err := filepath.Glob("fixtures/processPushEvent_*.json")
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

			var startReq *v1.StartJobRequest2
			client := mock.NewMockWerftServiceClient(mockCtrl)
			client.EXPECT().StartJob2(gomock.Any(), gomock.Any(), gomock.Any()).
				MinTimes(1).
				DoAndReturn(func(ctx context.Context, creq *v1.StartJobRequest2) (*v1.StartJobResponse, error) {
					startReq = creq
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
				Werft:  client,
				Config: &Config{},
			}
			if fixture.Event == nil {
				t.Fatal("broken fixture: no event")
			}
			plg.processPushEvent(fixture.Event)

			var expectation v1.StartJobRequest2
			goldenFN := strings.TrimSuffix(fn, filepath.Ext(fn)) + ".golden"
			if fc, err := os.ReadFile(goldenFN); err == nil {
				err = jsonpb.Unmarshal(bytes.NewReader(fc), &expectation)
				if err != nil {
					t.Fatal(err)
				}
			} else {
				var m jsonpb.Marshaler
				fc, err := m.MarshalToString(startReq)
				if err != nil {
					t.Fatal(err)
				}
				_ = ioutil.WriteFile(goldenFN, []byte(fc), 0644)
				t.Fatalf("no golden file present %s: wrote %s", fn, goldenFN)
			}

			if diff := cmp.Diff(expectation, *startReq); diff != "" {
				t.Errorf("processPushEvent() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestProcessPullRequestEditedEvent(t *testing.T) {
	type Fixture struct {
		Event        *github.PullRequestEvent
		ListResponse []*v1.JobStatus
	}

	fs, err := filepath.Glob("fixtures/processPullRequestEvent_*.json")
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

			var startReq *v1.StartJobRequest2
			client := mock.NewMockWerftServiceClient(mockCtrl)
			client.EXPECT().ListJobs(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes().
				DoAndReturn(func(ctx context.Context, in *v1.ListJobsRequest) (*v1.ListJobsResponse, error) {
					return &v1.ListJobsResponse{Result: fixture.ListResponse}, nil
				})
			client.EXPECT().StartJob2(gomock.Any(), gomock.Any(), gomock.Any()).
				AnyTimes().
				DoAndReturn(func(ctx context.Context, creq *v1.StartJobRequest2) (*v1.StartJobResponse, error) {
					startReq = creq
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
				Werft:    client,
				Config:   &Config{},
				testMode: true,
			}
			if fixture.Event == nil {
				t.Fatal("broken fixture: no event")
			}
			plg.processPullRequestEditedEvent(context.Background(), fixture.Event)

			var expectation *v1.StartJobRequest2
			goldenFN := strings.TrimSuffix(fn, filepath.Ext(fn)) + ".golden"
			if fc, err := os.ReadFile(goldenFN); err == nil && len(fc) > 0 {
				var ex v1.StartJobRequest2
				err = jsonpb.Unmarshal(bytes.NewReader(fc), &ex)
				if err != nil {
					t.Fatal(err)
				}
				expectation = &ex
			} else if os.IsNotExist(err) {
				var (
					m   jsonpb.Marshaler
					fc  string
					err error
				)
				if startReq != nil {
					fc, err = m.MarshalToString(startReq)
					if err != nil {
						t.Fatal(err)
					}
				}
				_ = ioutil.WriteFile(goldenFN, []byte(fc), 0644)
				t.Fatalf("no golden file present %s: wrote %s", fn, goldenFN)
			} else if err != nil {
				t.Fatal(err)
			}

			less := func(a, b *v1.Annotation) bool { return a.Key < b.Key }
			if diff := cmp.Diff(expectation, startReq, cmpopts.SortSlices(less)); diff != "" {
				t.Errorf("processPullRequestEditedEvent() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
