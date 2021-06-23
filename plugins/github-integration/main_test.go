package main

import (
	"testing"

	"github.com/google/go-cmp/cmp"
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
