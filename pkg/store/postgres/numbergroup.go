package postgres

import (
	"database/sql"

	"github.com/32leaves/werft/pkg/store"
)

// NumberGroup provides postgres backed number groups
type NumberGroup struct {
	DB *sql.DB
}

// NewNumberGroup creates a new SQL number group store
func NewNumberGroup(db *sql.DB) (*NumberGroup, error) {
	return &NumberGroup{DB: db}, nil
}

// Latest returns the latest number of a particular number group.
func (ngrp *NumberGroup) Latest(group string) (nr int, err error) {
	err = ngrp.DB.QueryRow(`
		SELECT val
		FROM   number_group
		WHERE  name = $1`,
		group,
	).Scan(&nr)
	if err == sql.ErrNoRows {
		return 0, store.ErrNotFound
	}
	return
}

// Next returns the next number in the group.
func (ngrp *NumberGroup) Next(group string) (nr int, err error) {
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
