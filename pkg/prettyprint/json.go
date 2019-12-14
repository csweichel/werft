package prettyprint

import "github.com/gogo/protobuf/jsonpb"

// JSONFormat formats everythign as JSON
const JSONFormat Format = "json"

func formatJSON(pp *Content) error {
	enc := &jsonpb.Marshaler{
		EnumsAsInts: false,
		Indent:      "  ",
	}
	return enc.Marshal(pp.Writer, pp.Obj)
}
