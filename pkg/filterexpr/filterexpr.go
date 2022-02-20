package filterexpr

import (
	"fmt"
	"strings"

	v2 "github.com/csweichel/werft/pkg/api/v2"
	"golang.org/x/xerrors"
)

// ErrMissingOp indicates that the expression was not complete
var ErrMissingOp = fmt.Errorf("missing operator")

// Parse parses a list of expressions
func Parse(exprs []string) ([]*v2.FilterTerm, error) {
	ops := map[string]v2.FilterOp{
		"==": v2.FilterOp_OP_EQUALS,
		"~=": v2.FilterOp_OP_CONTAINS,
		"|=": v2.FilterOp_OP_STARTS_WITH,
		"=|": v2.FilterOp_OP_ENDS_WITH,
	}

	res := make([]*v2.FilterTerm, len(exprs))
	for i, expr := range exprs {
		var (
			op  v2.FilterOp
			opn string
			neg bool
		)
		for k, v := range ops {
			if strings.Contains(expr, "!"+k) {
				op = v
				opn = "!" + k
				neg = true
				break
			}
			if strings.Contains(expr, k) {
				op = v
				opn = k
				break
			}
		}
		if opn == "" {
			return nil, ErrMissingOp
		}

		segs := strings.Split(expr, opn)
		field, val := strings.TrimSpace(segs[0]), strings.TrimSpace(segs[1])
		if field == "success" {
			if val == "true" {
				val = "1"
			} else {
				val = "0"
			}
		}
		if field == "phase" {
			phn := strings.ToUpper(fmt.Sprintf("PHASE_%s", val))
			if _, ok := v2.JobPhase_value[phn]; !ok {
				return nil, xerrors.Errorf("invalid phase: %s", val)
			}
		}

		res[i] = &v2.FilterTerm{
			Field:     field,
			Value:     val,
			Operation: op,
			Negate:    neg,
		}
	}

	return res, nil
}

// MatchesFilter returns true if the annotations are matched by the filter
func MatchesFilter(js *v2.JobStatus, filter []*v2.FilterExpression) (matches bool) {
	if len(filter) == 0 {
		return true
	}
	if js == nil {
		return false
	}

	idx := map[string]string{
		"name":  js.Name,
		"phase": strings.ToLower(strings.TrimPrefix(js.Phase.String(), "PHASE_")),
	}
	if js.Metadata != nil {
		idx["owner"] = js.Metadata.Owner
		idx["trigger"] = strings.ToLower(strings.TrimPrefix("TRIGGER_", js.Metadata.Trigger.String()))
		if js.Metadata.Repository != nil {
			idx["repo.owner"] = js.Metadata.Repository.Owner
			idx["repo.repo"] = js.Metadata.Repository.Repo
			idx["repo.host"] = js.Metadata.Repository.Host
			idx["repo.ref"] = js.Metadata.Repository.Ref
			idx["repo.rev"] = js.Metadata.Repository.Revision
		}
	}
	for _, at := range js.Metadata.Annotations {
		idx["annotation."+at.Key] = at.Value
	}

	matches = true
	for _, req := range filter {
		var tm bool
		for _, alt := range req.Terms {
			val, ok := idx[alt.Field]
			if !ok {
				continue
			}

			switch alt.Operation {
			case v2.FilterOp_OP_CONTAINS:
				tm = strings.Contains(val, alt.Value)
			case v2.FilterOp_OP_ENDS_WITH:
				tm = strings.HasSuffix(val, alt.Value)
			case v2.FilterOp_OP_EQUALS:
				tm = val == alt.Value
			case v2.FilterOp_OP_STARTS_WITH:
				tm = strings.HasPrefix(val, alt.Value)
			case v2.FilterOp_OP_EXISTS:
				tm = true
			}

			if alt.Negate {
				tm = !tm
			}

			if tm {
				break
			}
		}

		if !tm {
			matches = false
			break
		}
	}
	return matches
}
