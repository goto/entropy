package queries

import (
	"context"
	_ "embed"
)

func (q *Queries) Migrate(ctx context.Context, schema string) error {
	_, err := q.db.Exec(ctx, schema)
	return err
}
