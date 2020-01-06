package filterexpr

import (
	"fmt"
	"strings"

	v1 "github.com/32leaves/werft/pkg/api/v1"
	"golang.org/x/xerrors"
)

// ErrMissingOp indicates that the expression was not complete
var ErrMissingOp = fmt.Errorf("missing operator")

// Parse parses a list of expressions
func Parse(exprs []string) ([]*v1.FilterTerm, error) {
	ops := map[string]v1.FilterOp{
		"==": v1.FilterOp_OP_EQUALS,
		"~=": v1.FilterOp_OP_CONTAINS,
		"|=": v1.FilterOp_OP_STARTS_WITH,
		"=|": v1.FilterOp_OP_ENDS_WITH,
	}

	res := make([]*v1.FilterTerm, len(exprs))
	for i, expr := range exprs {
		var (
			op  v1.FilterOp
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
			if _, ok := v1.JobPhase_value[phn]; !ok {
				return nil, xerrors.Errorf("invalid phase: %s", val)
			}
		}

		res[i] = &v1.FilterTerm{
			Field:     field,
			Value:     val,
			Operation: op,
			Negate:    neg,
		}
	}

	return res, nil
}

// MatchesFilter returns true if the annotations are matched by the filter
func MatchesFilter(js *v1.JobStatus, filter []*v1.FilterExpression) (matches bool) {
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
			case v1.FilterOp_OP_CONTAINS:
				tm = strings.Contains(val, alt.Value)
			case v1.FilterOp_OP_ENDS_WITH:
				tm = strings.HasSuffix(val, alt.Value)
			case v1.FilterOp_OP_EQUALS:
				tm = val == alt.Value
			case v1.FilterOp_OP_STARTS_WITH:
				tm = strings.HasPrefix(val, alt.Value)
			case v1.FilterOp_OP_EXISTS:
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
