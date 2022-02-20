package filterexpr_test

import (
	"reflect"
	"testing"

	"github.com/alecthomas/repr"
	v2 "github.com/csweichel/werft/pkg/api/v2"
	"github.com/csweichel/werft/pkg/filterexpr"
)

func TestValidBasics(t *testing.T) {
	tests := []struct {
		Input  string
		Result *v2.FilterTerm
		Error  string
	}{
		{"foo==bar", &v2.FilterTerm{Field: "foo", Value: "bar", Operation: v2.FilterOp_OP_EQUALS, Negate: false}, ""},
		{"foo!==bar", &v2.FilterTerm{Field: "foo", Value: "bar", Operation: v2.FilterOp_OP_EQUALS, Negate: true}, ""},
		{"foo~=bar", &v2.FilterTerm{Field: "foo", Value: "bar", Operation: v2.FilterOp_OP_CONTAINS, Negate: false}, ""},
		{"foo!~=bar", &v2.FilterTerm{Field: "foo", Value: "bar", Operation: v2.FilterOp_OP_CONTAINS, Negate: true}, ""},
		{"foo|=bar", &v2.FilterTerm{Field: "foo", Value: "bar", Operation: v2.FilterOp_OP_STARTS_WITH, Negate: false}, ""},
		{"foo!|=bar", &v2.FilterTerm{Field: "foo", Value: "bar", Operation: v2.FilterOp_OP_STARTS_WITH, Negate: true}, ""},
		{"foo=|bar", &v2.FilterTerm{Field: "foo", Value: "bar", Operation: v2.FilterOp_OP_ENDS_WITH, Negate: false}, ""},
		{"foo!=|bar", &v2.FilterTerm{Field: "foo", Value: "bar", Operation: v2.FilterOp_OP_ENDS_WITH, Negate: true}, ""},
		{"success==true", &v2.FilterTerm{Field: "success", Value: "1", Operation: v2.FilterOp_OP_EQUALS, Negate: false}, ""},
		{"success==false", &v2.FilterTerm{Field: "success", Value: "0", Operation: v2.FilterOp_OP_EQUALS, Negate: false}, ""},
		{"success!==true", &v2.FilterTerm{Field: "success", Value: "1", Operation: v2.FilterOp_OP_EQUALS, Negate: true}, ""},
		{"success!==false", &v2.FilterTerm{Field: "success", Value: "0", Operation: v2.FilterOp_OP_EQUALS, Negate: true}, ""},
		{"trim == whitespace", &v2.FilterTerm{Field: "trim", Value: "whitespace", Operation: v2.FilterOp_OP_EQUALS, Negate: false}, ""},
		{"foo", nil, filterexpr.ErrMissingOp.Error()},
		{"phase==blabla", nil, "invalid phase: blabla"},
	}

	for _, test := range tests {
		res, err := filterexpr.Parse([]string{test.Input})
		if err != nil {
			if err.Error() != test.Error {
				t.Errorf("%s: %v != %v", test.Input, err, test.Error)
			}
			continue
		}

		if len(res) != 1 {
			t.Errorf("%s: resulted in NOT exactly one expression, but %v", test.Input, repr.String(res))
			continue
		}
		if !reflect.DeepEqual(res[0], test.Result) {
			t.Errorf("%s: expected %s but got %s", test.Input, repr.String(test.Result), repr.String(res[0]))
			continue
		}
	}
}

func TestMatchesFilter(t *testing.T) {
	md := &v2.JobMetadata{
		Owner:      "foo",
		Repository: &v2.Repository{},
	}
	tests := []struct {
		Job     *v2.JobStatus
		Expr    []*v2.FilterExpression
		Matches bool
	}{
		{
			&v2.JobStatus{Metadata: md, Phase: v2.JobPhase_PHASE_DONE},
			[]*v2.FilterExpression{{Terms: []*v2.FilterTerm{{Field: "phase", Value: "done", Operation: v2.FilterOp_OP_EQUALS}}}},
			true,
		},
		{
			&v2.JobStatus{Metadata: md, Name: "foobar.1"},
			[]*v2.FilterExpression{{Terms: []*v2.FilterTerm{{Field: "name", Value: "foobar", Operation: v2.FilterOp_OP_STARTS_WITH}}}},
			true,
		},
	}

	for idx, test := range tests {
		if filterexpr.MatchesFilter(test.Job, test.Expr) != test.Matches {
			t.Errorf("test %d failed", idx)
		}
	}
}
