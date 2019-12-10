package store

import "database/sql"

var nrGrpSchema = `
CREATE TABLE IF NOT EXISTS number_group (
	name varchar(255) NOT NULL PRIMARY KEY,
	val int NOT NULL
);
`

// SQLNumberGroup provides postgres backed number groups
type SQLNumberGroup struct {
	DB *sql.DB
}

// NewSQLNumberGroup creates a new SQL number group store
func NewSQLNumberGroup(db *sql.DB) (*SQLNumberGroup, error) {
	_, err := db.Exec(nrGrpSchema)
	if err != nil {
		return nil, err
	}

	return &SQLNumberGroup{DB: db}, nil
}

// Latest returns the latest number of a particular number group.
func (ngrp *SQLNumberGroup) Latest(group string) (nr int, err error) {
	err = ngrp.DB.QueryRow(`
		SELECT val
		FROM   number_group
		WHERE  name = $1`,
		group,
	).Scan(&nr)
	if err == sql.ErrNoRows {
		return 0, ErrNotFound
	}
	return
}

// Next returns the next number in the group.
func (ngrp *SQLNumberGroup) Next(group string) (nr int, err error) {
	err = ngrp.DB.QueryRow(`
		INSERT
		INTO   number_group (name, val)
		VALUES              ($1  , 0  )
		ON CONFLICT (name) DO UPDATE 
			SET val = number_group.val + 1
		RETURNING val`,
		group,
	).Scan(&nr)
	return
}
