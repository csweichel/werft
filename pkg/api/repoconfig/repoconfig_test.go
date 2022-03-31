package repoconfig_test

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/csweichel/werft/pkg/api/repoconfig"
	v1 "github.com/csweichel/werft/pkg/api/v1"
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
		Name        string
		Config      repoconfig.C
		Metadata    v1.JobMetadata
		Expectation string
	}{
		{
			Name: "all empty",
		},
		{
			Name: "empty config",
			Metadata: v1.JobMetadata{
				Owner:      "foo",
				Repository: &v1.Repository{Owner: "foo"},
				Trigger:    v1.JobTrigger_TRIGGER_MANUAL,
			},
		},
		{
			Name:        "default job",
			Config:      repoconfig.C{DefaultJob: "foo"},
			Expectation: "foo",
		},
		{
			Name: "basic rule",
			Config: repoconfig.C{
				DefaultJob: "foo",
				Rules:      []*repoconfig.JobStartRule{{Path: "bar"}},
			},
			Expectation: "bar",
		},
		{
			Name: "no match",
			Config: repoconfig.C{
				DefaultJob: "foo",
				Rules: []*repoconfig.JobStartRule{
					{
						Path: "bar",
						Expr: []*v1.FilterExpression{
							{
								Terms: []*v1.FilterTerm{
									{Field: "repo.ref", Value: "test", Operation: v1.FilterOp_OP_EQUALS},
								},
							},
						},
					},
				},
			},
			Expectation: "foo",
		},
		{
			Name: "rule match repo.ref",
			Config: repoconfig.C{
				DefaultJob: "foo",
				Rules: []*repoconfig.JobStartRule{
					{
						Path: "bar",
						Expr: []*v1.FilterExpression{
							{Terms: []*v1.FilterTerm{{Field: "repo.ref", Value: "test", Operation: v1.FilterOp_OP_EQUALS}}},
						},
					},
				},
			},
			Metadata: v1.JobMetadata{
				Repository: &v1.Repository{
					Ref: "test",
				},
			},
			Expectation: "bar",
		},
		{
			Name: "exclusive rule match",
			Config: repoconfig.C{
				Rules: []*repoconfig.JobStartRule{
					mustParseRule("path: bar\nmatchesAll:\n  - or: [\"repo.ref ~= refs/heads/\"]\n  - or: [\"trigger !== deleted\"]"),
				},
			},
			Metadata: v1.JobMetadata{
				Repository: &v1.Repository{
					Host:  "github.com",
					Owner: "csweichel",
					Repo:  "test-repo",
					Ref:   "refs/heads/cw/tbd",
				},
				Owner:   "csweichel",
				Trigger: v1.JobTrigger_TRIGGER_DELETED,
			},
			Expectation: "",
		},
	}

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			if test.Name == "exclusive rule match" {
				fmt.Println("foo")
			}
			act := test.Config.TemplatePath(&test.Metadata)
			if act != test.Expectation {
				t.Errorf("expected %s, actual %s", test.Expectation, act)
			}
		})
	}
}

func mustParseRule(exp string) *repoconfig.JobStartRule {
	var res repoconfig.JobStartRule
	err := yaml.Unmarshal([]byte(exp), &res)
	if err != nil {
		panic(err)
	}
	fmt.Printf("parsed rule: %v\n", res)
	return &res
}
