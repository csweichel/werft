package prettyprint

import (
	"gopkg.in/yaml.v3"
)

// YAMLFormat formats everythign as YAML
const YAMLFormat Format = "yaml"

func formatYAML(pp *Content) error {
	err := yaml.NewEncoder(pp.Writer).Encode(pp.Obj)
	if err != nil {
		return err
	}

	return nil
}
