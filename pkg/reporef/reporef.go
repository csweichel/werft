package reporef

import (
	"strings"

	v2 "github.com/csweichel/werft/pkg/api/v2"
	"golang.org/x/xerrors"
)

// Parse interprets a string pointing to a (GitHub) repository.
// We expect the string to be in the form of:
//    (host)/owner/repo(:ref|@sha)
func Parse(spec string) (*v2.Repository, error) {
	if strings.Contains(spec, ":") {
		segs := strings.Split(spec, ":")
		rep, ref := segs[0], segs[1]
		repo, err := parseRep(rep)
		if err != nil {
			return nil, err
		}
		repo.Ref = ref
		return repo, nil
	}
	if strings.Contains(spec, "@") {
		segs := strings.Split(spec, "@")
		rep, rev := segs[0], segs[1]
		repo, err := parseRep(rep)
		if err != nil {
			return nil, err
		}
		repo.Revision = rev
		return repo, nil
	}
	return parseRep(spec)
}

func parseRep(rep string) (*v2.Repository, error) {
	segs := strings.Split(rep, "/")
	if len(segs) < 2 || len(segs) > 3 {
		return nil, xerrors.Errorf("invalid repository spec")
	}

	res := &v2.Repository{}
	if len(segs) == 3 {
		res.Host = segs[0]
		segs = segs[1:]
	}
	res.Owner = segs[0]
	res.Repo = segs[1]
	return res, nil
}
