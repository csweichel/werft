package store

import (
	"bytes"
	"context"
	"io"
	"io/ioutil"
	"strings"
	"sync"

	v1 "github.com/32leaves/keel/pkg/api/v1"
)

// NewInMemoryLogStore provides a new log store which stores its logs in memory
func NewInMemoryLogStore() Logs {
	return &inMemoryLogStore{
		logs: make(map[string]*log),
	}
}

// inMemoryLogStore implements a log store in memory
type inMemoryLogStore struct {
	logs map[string]*log
	mu   sync.RWMutex
}

type log struct {
	Data *bytes.Buffer
	Done bool
}

// Place writes to this store
func (s *inMemoryLogStore) Place(ctx context.Context, id string, src io.Reader) error {
	s.mu.Lock()
	if _, ok := s.logs[id]; ok {
		s.mu.Unlock()
		return ErrAlreadyExists
	}

	lg := &log{Data: bytes.NewBuffer(nil)}

	s.logs[id] = lg
	s.mu.Unlock()

	_, err := io.Copy(lg.Data, src)
	lg.Done = true
	if err != nil {
		return err
	}

	return nil
}

// Read reads from this store
func (s *inMemoryLogStore) Read(ctx context.Context, id string) (io.ReadCloser, error) {
	s.mu.RLock()
	defer s.mu.Unlock()

	l, ok := s.logs[id]
	if !ok || !l.Done {
		return nil, ErrNotFound
	}

	return ioutil.NopCloser(l.Data), nil
}

// NewInMemoryJobStore creates a new in-memory job store
func NewInMemoryJobStore() Jobs {
	return &inMemoryJobStore{
		jobs: make(map[string]v1.JobStatus),
	}
}

type inMemoryJobStore struct {
	jobs map[string]v1.JobStatus
	mu   sync.RWMutex
}

// Store stores job information in the store.
// Storing a job whose name we already have in store will override the previously
// stored job.
func (s *inMemoryJobStore) Store(ctx context.Context, job v1.JobStatus) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.jobs[job.Name] = job
	return nil
}

// Retrieves a particular job bassd on its name.
// If the job is unknown we'll return ErrNotFound.
func (s *inMemoryJobStore) Get(ctx context.Context, name string) (*v1.JobStatus, error) {
	s.mu.RLock()
	job, ok := s.jobs[name]
	s.mu.Unlock()

	if !ok {
		return nil, ErrNotFound
	}

	return &job, nil
}

// Searches for jobs based on their annotations
func (s *inMemoryJobStore) Find(ctx context.Context, filter []*v1.FilterExpression, order []*v1.OrderExpression, start, limit int) (slice []v1.JobStatus, total int, err error) {
	var res []v1.JobStatus
	for _, js := range s.jobs {
		if !MatchesFilter(js.Metadata, filter) {
			continue
		}

		res = append(res, js)
	}
	return res, len(res), nil
}

// MatchesFilter returns true if the annotations are matched by the filter
func MatchesFilter(base *v1.JobMetadata, filter []*v1.FilterExpression) (matches bool) {
	if base == nil {
		if len(filter) == 0 {
			return true
		}

		return false
	}

	idx := map[string]string{
		"owner":      base.Owner,
		"repo.owner": base.Repository.Owner,
		"repo.repo":  base.Repository.Repo,
		"repo.host":  base.Repository.Host,
		"repo.ref":   base.Repository.Ref,
		"trigger":    strings.ToLower(strings.TrimPrefix("TRIGGER_", base.Trigger.String())),
	}
	for _, at := range base.Annotations {
		idx["annotation."+at.Key] = at.Value
	}

	matches = true
	for _, req := range filter {
		var tm bool
		for _, alt := range req.Terms {
			val, ok := idx[alt.Value]
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
				tm = strings.HasSuffix(val, alt.Value)
			case v1.FilterOp_OP_EXISTS:
				tm = true
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
