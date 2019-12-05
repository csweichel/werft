package store

import (
	"context"
	"fmt"
	"io"

	v1 "github.com/32leaves/werft/pkg/api/v1"
)

var (
	// ErrNotFound is returned by Read if something isn't found
	ErrNotFound = fmt.Errorf("not found")

	// ErrAlreadyExists is returned when attempting to place something which already exists
	ErrAlreadyExists = fmt.Errorf("exists already")
)

// Logs provides access to the logstore
type Logs interface {
	// Places a logfile in this store.
	// This function does not return until the reader returns EOF.
	Place(ctx context.Context, id string) (io.WriteCloser, error)

	// Read retrieves a log file from this store.
	// Returns ErrNotFound if the log file isn't found.
	// Callers are supposed to close the reader once done.
	// Reading from logs currently being written is supported.
	Read(ctx context.Context, id string) (io.ReadCloser, error)
}

// Jobs provides access to past jobs
type Jobs interface {
	// Store stores job information in the store.
	// Storing a job whose name we already have in store will override the previously
	// stored job.
	Store(ctx context.Context, job v1.JobStatus) error

	// Retrieves a particular job bassd on its name.
	// If the job is unknown we'll return ErrNotFound.
	Get(ctx context.Context, name string) (*v1.JobStatus, error)

	// Searches for jobs based on their annotations. If filter is empty no filter is applied.
	// If limit is 0, no limit is applied.
	Find(ctx context.Context, filter []*v1.FilterExpression, order []*v1.OrderExpression, start, limit int) (slice []v1.JobStatus, total int, err error)
}
