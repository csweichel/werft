package repoconfig_test

import (
	"encoding/json"
	"testing"

	"github.com/csweichel/werft/pkg/api/repoconfig"
	v2 "github.com/csweichel/werft/pkg/api/v2"
	"gopkg.in/yaml.v3"
)

func TestUnmarshalC(t *testing.T) {
	tests := []struct {
		Source      string
		Expectation string
	}{
		{`defaultJob: "foo.yaml"`, `{"DefaultJob":"foo.yaml","Rules":null}`},
		{
			`rules:
- path: ""
  matchesAll:
  - or: ["repo.ref ~= refs/tags/"]
- path: ""
  matchesAll:
  - or: ["repo.ref !~= refs/branches/"]`,
			`{"DefaultJob":"","Rules":[{"Path":"","Expr":[{"terms":[{"field":"repo.ref","value":"refs/tags/","operation":3}]}]},{"Path":"","Expr":[{"terms":[{"field":"repo.ref","value":"refs/branches/","operation":3,"negate":true}]}]}]}`,
		},
		{
			`rules:
- path: "foo.yaml"
  matchesAll:
  - or:
    - "repo.ref ~= refs/branches/"
  - or:
    - "name !~= 0"
`, `{"DefaultJob":"","Rules":[{"Path":"foo.yaml","Expr":[{"terms":[{"field":"repo.ref","value":"refs/branches/","operation":3}]},{"terms":[{"field":"name","value":"0","operation":3,"negate":true}]}]}]}`,
		},
	}

	for idx, test := range tests {
		var c repoconfig.C
		err := yaml.Unmarshal([]byte(test.Source), &c)
		if err != nil {
			t.Errorf("test %d: %v", idx, err)
			continue
		}

		act, err := json.Marshal(c)
		if err != nil {
			t.Errorf("test %d: %v", idx, err)
			continue
		}

		if string(act) != test.Expectation {
			t.Errorf("test %d: did not match expectation.\nExpected: %s\nActual: %s\n", idx, test.Expectation, string(act))
		}
	}
}

func TestTemplatePath(t *testing.T) {
	tests := []struct {
		C repoconfig.C
		M v2.JobMetadata
		E string
	}{
		{repoconfig.C{}, v2.JobMetadata{}, ""},
		{repoconfig.C{}, v2.JobMetadata{Owner: "foo", Repository: &v2.Repository{Owner: "foo"}, Trigger: v2.JobTrigger_TRIGGER_MANUAL}, ""},
		{repoconfig.C{DefaultJob: "foo"}, v2.JobMetadata{}, "foo"},
		{repoconfig.C{DefaultJob: "foo", Rules: []*repoconfig.JobStartRule{&repoconfig.JobStartRule{Path: "bar"}}}, v2.JobMetadata{}, "bar"},
		{
			repoconfig.C{
				DefaultJob: "foo",
				Rules: []*repoconfig.JobStartRule{
					&repoconfig.JobStartRule{
						Path: "bar",
						Expr: []*v2.FilterExpression{
							&v2.FilterExpression{Terms: []*v2.FilterTerm{&v2.FilterTerm{Field: "repo.ref", Value: "test", Operation: v2.FilterOp_OP_EQUALS}}},
						},
					},
				},
			},
			v2.JobMetadata{},
			"foo",
		},
		{
			repoconfig.C{
				DefaultJob: "foo",
				Rules: []*repoconfig.JobStartRule{
					&repoconfig.JobStartRule{
						Path: "bar",
						Expr: []*v2.FilterExpression{
							&v2.FilterExpression{Terms: []*v2.FilterTerm{&v2.FilterTerm{Field: "repo.ref", Value: "test", Operation: v2.FilterOp_OP_EQUALS}}},
						},
					},
				},
			},
			v2.JobMetadata{
				Repository: &v2.Repository{
					Ref: "test",
				},
			},
			"bar",
		},
	}

	for idx, test := range tests {
		act := test.C.TemplatePath(&test.M)
		if act != test.E {
			t.Errorf("test %d: expected %s, actual %s", idx, test.E, act)
		}
	}
}
