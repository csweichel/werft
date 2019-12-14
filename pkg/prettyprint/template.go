package prettyprint

import (
	"text/tabwriter"
	"text/template"
	"time"

	"github.com/golang/protobuf/ptypes"
	tspb "github.com/golang/protobuf/ptypes/timestamp"
)

// TemplateFormat uses Go templates and tabwriter for formatting content
const TemplateFormat Format = "template"

func formatTemplate(pp *Content) error {
	tmpl, err := template.
		New("prettyprint").
		Funcs(map[string]interface{}{
			"toRFC3339": func(t *tspb.Timestamp) string {
				ts, err := ptypes.Timestamp(t)
				if err != nil {
					return err.Error()
				}
				return ts.Format(time.RFC3339)
			},
		}).
		Parse(pp.Template)
	if err != nil {
		return err
	}

	w := tabwriter.NewWriter(pp.Writer, 8, 8, 8, ' ', 0)
	if err := tmpl.Execute(w, pp.Obj); err != nil {
		return err
	}
	if err := w.Flush(); err != nil {
		return err
	}
	return nil
}
