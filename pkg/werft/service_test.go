package werft

import "testing"

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
