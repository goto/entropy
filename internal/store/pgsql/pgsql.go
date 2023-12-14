package pgsql

import (
	"context"
	"database/sql"
	_ "embed"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	_ "github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/goto/entropy/core/resource"
	"github.com/goto/entropy/internal/store/pgsql/queries"
	"github.com/goto/entropy/pkg/errors"
)

//go:generate sqlc generate

// schema represents the storage schema.
// Note: Update the constants above if the table name is changed.
//
//go:embed schema.sql
var schema string

type Store struct {
	qu              *queries.Queries
	pgx             *pgx.Conn
	extendInterval  time.Duration
	refreshInterval time.Duration
}

func (st *Store) Migrate(ctx context.Context) error {
	err := st.qu.Migrate(ctx, schema)
	return err
}

func (st *Store) Close() error { return st.pgx.Close(context.Background()) }

// Open returns store instance backed by PostgresQL.
func Open(conStr string, refreshInterval, extendInterval time.Duration) (*Store, error) {
	conn, err := pgx.Connect(context.Background(), conStr)
	if err != nil {
		return nil, err
	} else if err := conn.Ping(context.Background()); err != nil {
		_ = conn.Close(context.Background())
		return nil, err
	}

	if refreshInterval >= extendInterval {
		return nil, errors.New("refreshInterval must be lower than extendInterval")
	}

	return &Store{
		qu:              queries.New(conn),
		pgx:             conn,
		extendInterval:  extendInterval,
		refreshInterval: refreshInterval,
	}, nil
}

func translateSQLErr(err error) error {
	if errors.Is(err, sql.ErrNoRows) {
		return errors.ErrNotFound.WithCausef(err.Error())
	}

	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		// Refer http://www.postgresql.org/docs/9.3/static/errcodes-appendix.html
		switch pgErr.Code {
		case "unique_violation":
			return errors.ErrConflict.WithCausef(err.Error())

		default:
			return errors.ErrInternal.WithCausef(err.Error())
		}
	}

	return err
}

func tagsToLabelMap(tags []string) map[string]string {
	const keyValueParts = 2

	labels := map[string]string{}
	for _, tag := range tags {
		parts := strings.SplitN(tag, "=", keyValueParts)
		key, val := parts[0], parts[1]
		labels[key] = val
	}
	return labels
}

func labelToTag(k, v string) string {
	return fmt.Sprintf("%s=%s", k, v)
}

type TxFunc func(ctx context.Context, tx pgx.Tx) error

func withinTx(ctx context.Context, db *pgx.Conn, readOnly bool, fns ...TxFunc) error {
	var opts pgx.TxOptions
	if readOnly {
		opts.AccessMode = pgx.ReadOnly
	} else {
		opts.AccessMode = pgx.ReadWrite
	}

	tx, err := db.BeginTx(ctx, opts)
	if err != nil {
		return err
	}

	for _, fn := range fns {
		if err := fn(ctx, tx); err != nil {
			_ = tx.Rollback(ctx)
			return err
		}
	}

	return tx.Commit(ctx)
}

func runAllHooks(ctx context.Context, hooks []resource.MutationHook) error {
	for _, hook := range hooks {
		if err := hook(ctx); err != nil {
			return err
		}
	}
	return nil
}
