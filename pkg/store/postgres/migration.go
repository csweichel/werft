package postgres

import (
	"database/sql"
	"os"

	rice "github.com/GeertJohan/go.rice"
	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source"
	"github.com/golang-migrate/migrate/v4/source/godoc_vfs"
	log "github.com/sirupsen/logrus"
	"golang.org/x/tools/godoc/vfs/mapfs"
	"golang.org/x/xerrors"
)

// Migrate ensures that the database has the current schema required for using any of the postgres storage
func Migrate(db *sql.DB) error {
	fs, err := getMigrations(db)
	if err != nil {
		return err
	}

	driver, err := postgres.WithInstance(db, &postgres.Config{})
	if err != nil {
		return err
	}

	mig, err := migrate.NewWithInstance("godoc-vfs", fs, "postgres", driver)
	if err != nil {
		return err
	}
	mig.Log = &logrusAdapter{}

	err = mig.Up()
	if err != nil && err != migrate.ErrNoChange {
		return xerrors.Errorf("error during migration: %w", err)
	}

	return nil
}

func getMigrations(db *sql.DB) (source.Driver, error) {
	box, err := rice.FindBox("migrations")
	if err != nil {
		return nil, err
	}
	migs := make(map[string]string)
	err = box.Walk("", func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}

		migs[path], err = box.String(path)
		if err != nil {
			return xerrors.Errorf("cannot read from migration box: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, xerrors.Errorf("cannot list migrations: %w", err)
	}
	fs, err := godoc_vfs.WithInstance(mapfs.New(migs), "")
	if err != nil {
		return nil, err
	}

	return fs, nil
}

type logrusAdapter struct{}

func (*logrusAdapter) Printf(format string, args ...interface{}) {
	log.WithField("migration", true).Debugf(format, args...)
}

func (*logrusAdapter) Verbose() bool {
	return true
}
