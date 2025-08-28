package postgres

import (
	"context"
	"database/sql"
	_ "embed"
	"time"

	"github.com/jmoiron/sqlx"
	"go.nhat.io/otelsql"
	semconv "go.opentelemetry.io/otel/semconv/v1.4.0"

	"github.com/goto/entropy/pkg/errors"
)

const (
	tableResources            = "resources"
	tableResourceTags         = "resource_tags"
	tableResourceDependencies = "resource_dependencies"
	columnResourceID          = "resource_id"

	tableRevisions    = "revisions"
	tableRevisionTags = "revision_tags"
	columnRevisionID  = "revision_id"
)

// schema represents the storage schema.
// Note: Update the constants above if the table name is changed.
//
//go:embed schema.sql
var schema string

type Store struct {
	db              *sqlx.DB
	extendInterval  time.Duration
	refreshInterval time.Duration
	config          Config
}

type Config struct {
	PaginationSizeDefault int32
	PaginationPageDefault int32
}

func (st *Store) Migrate(ctx context.Context) error {
	_, err := st.db.ExecContext(ctx, schema)
	return err
}

func (st *Store) Close() error { return st.db.Close() }

// Open returns store instance backed by PostgresQL.
func Open(conStr string, refreshInterval, extendInterval time.Duration, paginationSizeDefault, paginationPageDefault int32) (*Store, error) {
	driverName, err := otelsql.Register("postgres",
		otelsql.AllowRoot(),
		otelsql.TraceQueryWithoutArgs(),
		otelsql.TraceRowsClose(),
		otelsql.TraceRowsAffected(),
		otelsql.WithSystem(semconv.DBSystemPostgreSQL), // Optional.
	)
	if err != nil {
		return nil, err
	}

	db, err := sql.Open(driverName, conStr)
	if err != nil {
		return nil, err
	}

	dbSQLx := sqlx.NewDb(db, "postgres")

	if refreshInterval >= extendInterval {
		return nil, errors.New("refreshInterval must be lower than extendInterval")
	}

	return &Store{
		db:              dbSQLx,
		extendInterval:  extendInterval,
		refreshInterval: refreshInterval,
		config: Config{
			PaginationSizeDefault: paginationSizeDefault,
			PaginationPageDefault: paginationPageDefault,
		},
	}, nil
}
