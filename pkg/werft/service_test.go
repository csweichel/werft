package werft

import "testing"

import "github.com/google/go-cmp/cmp"

func TestCleanupPodName(t *testing.T) {
	tests := []struct {
		Input       string
		Expectation string
	}{
		{"this-is-an-invalid-podname-.33", "this-is-an-invalid-podnamea.33"},
		{"", "unknown"},
		// This test case happens to be shortened s.t. it ends with a dash, which is invalid.
		// The cleanup function should not let that happen.
		{"this-is-way-too-long-this-is-way-too-long-this-is-way-too-long", "this-is-way-too-long-this-is-way-too-long-this-is-way-tooa"},
	}

	for _, test := range tests {
		t.Run(test.Input, func(t *testing.T) {
			act := cleanupPodName(test.Input)
			if act != test.Expectation {
				t.Errorf("unexpected result: \"%s\"; expected \"%s\"", act, test.Expectation)
			}
		})
	}
}

func TestParseAnnotations(t *testing.T) {
	tests := []struct {
		Name     string
		Input    string
		Expected map[string]string
	}{
		{
			Name:  "empty string",
			Input: "",
		},
		{
			Name:  "unrelated content",
			Input: "Something unrelated",
		},
		{
			Name:     "werft annotation",
			Input:    "/werft foobar",
			Expected: map[string]string{"foobar": ""},
		},
		{
			Name:     "werft annotation with value",
			Input:    "/werft foobar=value",
			Expected: map[string]string{"foobar": "value"},
		},
		{
			Name:     "werft annotation with checkbox",
			Input:    "- [x] /werft foobar",
			Expected: map[string]string{"foobar": ""},
		},
		{
			Name:     "werft annotation with checkbox",
			Input:    "- [x]    /werft foobar=value",
			Expected: map[string]string{"foobar": "value"},
		},
		{
			Name:  "werft annotation with unchecked list checkbox",
			Input: "- [ ] /werft foobar",
		},
		{
			Name:     "mixed werft annotation",
			Input:    "hello world\n  /werft foo=bar",
			Expected: map[string]string{"foo": "bar"},
		},
		{
			Name:     "werft annotation with complex value",
			Input:    "/werft foobar=this=is=another/value 12,3,4,5",
			Expected: map[string]string{"foobar": "this=is=another/value 12,3,4,5"},
		},
		{
			Name:     "werft annotation with empty value",
			Input:    "/werft foobar=",
			Expected: map[string]string{"foobar": ""},
		},
	}

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			res := parseAnnotations(test.Input)
			if diff := cmp.Diff(test.Expected, res); diff != "" {
				t.Errorf("MakeGatewayInfo() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
