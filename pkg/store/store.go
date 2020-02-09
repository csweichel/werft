package store

import (
	"context"
	"fmt"
	"io"

	v1 "github.com/csweichel/werft/pkg/api/v1"
)

var (
	// ErrNotFound is returned by Read if something isn't found
	ErrNotFound = fmt.Errorf("not found")

	// ErrAlreadyExists is returned when attempting to place something which already exists
	ErrAlreadyExists = fmt.Errorf("exists already")
)

// Logs provides access to the logstore
type Logs interface {
	// Open places a logfile in this store.
	// The caller is expected to close this writer when the task is complete.
	// If the logfile is already open we'll return an error.
	Open(id string) (io.WriteCloser, error)

	// Write writes to a previously placed logfile.
	// If the logfile is unknown, we'll return an error.
	Write(id string) (io.Writer, error)

	// Read retrieves a log file from this store.
	// Returns ErrNotFound if the log file isn't found.
	// Callers are supposed to close the reader once done.
	// Reading from logs currently being written is supported.
	Read(id string) (io.ReadCloser, error)
}

// Jobs provides access to past jobs
type Jobs interface {
	// Store stores job information in the store.
	// Storing a job whose name we already have in store will override the previously
	// stored job.
	Store(ctx context.Context, job v1.JobStatus) error

	// StoreJobSpec stores job YAML data.
	StoreJobSpec(name string, data []byte) error

	// Retrieves a particular job bassd on its name.
	// If the job is unknown we'll return ErrNotFound.
	Get(ctx context.Context, name string) (*v1.JobStatus, error)

	// Get retrieves previously stored job spec data
	GetJobSpec(name string) (data []byte, err error)

	// Searches for jobs based on their annotations. If filter is empty no filter is applied.
	// If limit is 0, no limit is applied.
	Find(ctx context.Context, filter []*v1.FilterExpression, order []*v1.OrderExpression, start, limit int) (slice []v1.JobStatus, total int, err error)
}

// NumberGroup enables to atomic generation and storage of numbers.
// This is used for build numbering
type NumberGroup interface {
	// Latest returns the latest number of a particular number group.
	// Returns ErrNotFound if the group does not exist. A zero result is a valid
	// number in a group and does not indicate its non-existence.
	Latest(group string) (nr int, err error)

	// Next returns the next number in the group. If the group did not exist prior
	// to this call it is created. This function is thread-safe and atomic.
	Next(group string) (nr int, err error)
}
