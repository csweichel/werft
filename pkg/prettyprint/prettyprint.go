package prettyprint

import (
	"fmt"
	"io"

	"github.com/gogo/protobuf/proto"
)

// Format defines the kind of pretty-printing format we want to use
type Format string

// HasFormat returns true if the format is supported
func HasFormat(fmt Format) bool {
	_, ok := formatter[fmt]
	return ok
}

const (
	// StringFormat uses the Go-builtin stringification for printing
	StringFormat Format = "string"
)

type formatterFunc func(*Content) error

var formatter = map[Format]formatterFunc{
	StringFormat:   formatString,
	TemplateFormat: formatTemplate,
	JSONFormat:     formatJSON,
	YAMLFormat:     formatYAML,
}

func formatString(pp *Content) error {
	_, err := fmt.Fprintf(pp.Writer, "%s", pp.Obj)
	return err
}

// Content is pretty-printable content
type Content struct {
	Obj      proto.Message
	Format   Format
	Writer   io.Writer
	Template string
}

// Print outputs the content to its writer in the given format
func (pp *Content) Print() error {
	formatter, ok := formatter[pp.Format]
	if !ok {
		return fmt.Errorf("Unknown format: %s", pp.Format)
	}

	return formatter(pp)
}
