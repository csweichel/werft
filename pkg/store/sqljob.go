package store

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	v1 "github.com/32leaves/werft/pkg/api/v1"
	"github.com/gogo/protobuf/jsonpb"
	log "github.com/sirupsen/logrus"
	"golang.org/x/xerrors"
)

var jobSchema = `
CREATE TABLE IF NOT EXISTS job_status (
	id SERIAL PRIMARY KEY,
	name varchar(255) NOT NULL UNIQUE,
	data text NOT NULL,
	owner varchar(255) NULL,
	phase VARCHAR(255) NOT NULL,
	repo_owner varchar(255) NULL,
	repo_repo varchar(255) NULL,
	repo_host varchar(255) NULL,
	repo_ref varchar(255) NULL,
	trigger_src varchar(255) NULL,
	success int not null,
	created int not null
);

CREATE TABLE IF NOT EXISTS annotations (
	job_id INT NOT NULL,
	name varchar(255) NOT NULL,
	value text NULL,
	CONSTRAINT job_annotation UNIQUE(job_id, name)
);
`

// SQLJobStore stores jobs in a Postgres database
type SQLJobStore struct {
	DB *sql.DB
}

// NewSQLJobStore creates a new SQL job store
func NewSQLJobStore(db *sql.DB) (*SQLJobStore, error) {
	_, err := db.Exec(jobSchema)
	if err != nil {
		return nil, err
	}

	return &SQLJobStore{DB: db}, nil
}

// Store stores job information in the store.
func (s *SQLJobStore) Store(ctx context.Context, job v1.JobStatus) error {
	marshaler := &jsonpb.Marshaler{
		EnumsAsInts: true,
	}
	serializedJob, err := marshaler.MarshalToString(&job)
	if err != nil {
		return err
	}

	success := 0
	if job.Conditions.Success {
		success = 1
	}

	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	var jobID int
	err = tx.QueryRow(`
		INSERT
		INTO   job_status (name, data, owner, phase, repo_owner, repo_repo, repo_host, repo_ref, trigger_src, success, created)
		VALUES            ($1  , $2  , $3   , $4   , $5        , $6       , $7       , $8      , $9         , $10,     $11    ) 
		ON CONFLICT (name) DO UPDATE 
			SET data = $2, owner = $3, phase = $4, repo_owner = $5, repo_repo = $6, repo_host = $7, repo_ref = $8, trigger_src = $9, success = $10, created = $11
		RETURNING id`,
		job.Name,
		serializedJob,
		job.Metadata.Owner,
		strings.ToLower(job.Phase.String()),
		job.Metadata.Repository.Owner,
		job.Metadata.Repository.Repo,
		job.Metadata.Repository.Host,
		job.Metadata.Repository.Ref,
		strings.ToLower(strings.TrimPrefix("TRIGGER_", job.Metadata.Trigger.String())),
		success,
		job.Metadata.Created.Seconds,
	).Scan(&jobID)
	if err != nil {
		tx.Rollback()
		return err
	}
	for _, annotation := range job.Metadata.Annotations {
		_, err := tx.Exec(`
		INSERT
		INTO   annotations (job_id, name, value)
		VALUES             ($1    , $2  , $3   )
		ON CONFLICT ON CONSTRAINT job_annotation DO UPDATE
			SET value = $3
		`, jobID, annotation.Key, annotation.Value)
		if err != nil {
			tx.Rollback()
			return err
		}
	}

	err = tx.Commit()
	if err != nil {
		return err
	}

	return nil
}

// Get retrieves a particular job bassd on its name.
func (s *SQLJobStore) Get(ctx context.Context, name string) (*v1.JobStatus, error) {
	var data string
	err := s.DB.QueryRow("SELECT data FROM job_status WHERE name = $1", name).Scan(&data)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}

	var res v1.JobStatus
	err = jsonpb.UnmarshalString(data, &res)
	if err != nil {
		return nil, err
	}

	return &res, nil
}

// Find searches for jobs based on their annotations. If filter is empty no filter is applied.
func (s *SQLJobStore) Find(ctx context.Context, filter []*v1.FilterExpression, order []*v1.OrderExpression, start, limit int) (slice []v1.JobStatus, total int, err error) {
	fieldMap := map[string]string{
		"name":       "name",
		"owner":      "owner",
		"phase":      "phase",
		"repo.owner": "repo_owner",
		"repo.repo":  "repo_repo",
		"repo.host":  "repo_host",
		"repo.ref":   "repo_ref",
		"trigger":    "trigger",
		"success":    "success",
		"created":    "created",
	}

	var (
		whereExps []string
		args      []interface{}
	)
	for _, f := range filter {
		var terms []string
		for _, t := range f.Terms {
			field, ok := fieldMap[t.Field]
			if !ok {
				return nil, 0, xerrors.Errorf("unknown field %s", t.Field)
			}

			var op string
			switch t.Operation {
			case v1.FilterOp_OP_CONTAINS:
				op = "LIKE '%' || ? || '%'"
			case v1.FilterOp_OP_ENDS_WITH:
				op = "LIKE '%' || ?"
			case v1.FilterOp_OP_EQUALS:
				op = "= ?"
			case v1.FilterOp_OP_STARTS_WITH:
				op = "LIKE ? || '%'"
			case v1.FilterOp_OP_EXISTS:
				op = "IS NOT NULL"
			default:
				return nil, 0, xerrors.Errorf("unknown operation %v", t.Operation)
			}
			expr := fmt.Sprintf("%s %s", field, op)
			terms = append(terms, expr)
			args = append(args, t.Value)
		}
		expr := fmt.Sprintf("(%s)", strings.Join(terms, " OR "))
		whereExps = append(whereExps, expr)
	}
	whereExp := strings.Join(whereExps, " AND ")
	if whereExp != "" {
		whereExp = "WHERE " + whereExp
		prev := ""
		for i := 1; prev != whereExp; i++ {
			prev = whereExp
			whereExp = strings.Replace(whereExp, "?", fmt.Sprintf("$%d", i), 1)
		}
	}

	var orderExps []string
	for _, o := range order {
		field, ok := fieldMap[o.Field]
		if !ok {
			return nil, 0, xerrors.Errorf("unknown field %s", o.Field)
		}

		dir := "DESC"
		if o.Ascending {
			dir = "ASC"
		}
		orderExps = append(orderExps, fmt.Sprintf("%s %s", field, dir))
	}
	var orderExp string
	if len(orderExps) > 0 {
		orderExp = fmt.Sprintf("ORDER BY %s", strings.Join(orderExps, ", "))
	}

	limitExp := "ALL"
	if limit > 0 {
		limitExp = fmt.Sprintf("%d", limit)
	}

	countQuery := fmt.Sprintf("SELECT COUNT(1) FROM job_status %s", whereExp)
	log.WithField("query", countQuery).Debug("running query")
	err = s.DB.QueryRow(countQuery, args...).Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	query := fmt.Sprintf("SELECT data FROM job_status %s %s LIMIT %s OFFSET %d", whereExp, orderExp, limitExp, start)
	log.WithField("query", query).Debug("running query")
	rows, err := s.DB.Query(query, args...)
	if err != nil {
		return nil, 0, err
	}

	var result []v1.JobStatus
	for rows.Next() {
		var data string
		err = rows.Scan(&data)
		if err != nil {
			return nil, 0, err
		}

		var res v1.JobStatus
		err = jsonpb.UnmarshalString(data, &res)
		if err != nil {
			return nil, 0, err
		}

		result = append(result, res)
	}
	if rows.Err() != nil {
		return nil, 0, err
	}

	return result, total, nil
}
