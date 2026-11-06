package postgres

import (
	"context"
	"database/sql"
	"time"

	sq "github.com/Masterminds/squirrel"
	"go.nhat.io/otelsql"
	"go.opentelemetry.io/otel/attribute"
	semconv "go.opentelemetry.io/otel/semconv/v1.4.0"

	"github.com/goto/entropy/core/addon"
	"github.com/goto/entropy/pkg/errors"
)

const (
	tableAddons       = "addons"
	tableAddonConfigs = "addon_configs"
)

var addonColumns = []string{
	"id", "urn", "parent_urn", "type", "spec", "output", "status", "last_error",
	"spec_version", "created_at", "updated_at", "created_by", "updated_by",
}

type addonModel struct {
	ID          int64     `db:"id"`
	URN         string    `db:"urn"`
	ParentURN   string    `db:"parent_urn"`
	Type        string    `db:"type"`
	Spec        []byte    `db:"spec"`
	Output      []byte    `db:"output"`
	Status      string    `db:"status"`
	LastError   string    `db:"last_error"`
	SpecVersion int64     `db:"spec_version"`
	CreatedAt   time.Time `db:"created_at"`
	UpdatedAt   time.Time `db:"updated_at"`
	CreatedBy   string    `db:"created_by"`
	UpdatedBy   string    `db:"updated_by"`
}

func (am addonModel) toAddon() addon.Addon {
	return addon.Addon{
		ID:          am.ID,
		URN:         am.URN,
		ParentURN:   am.ParentURN,
		Type:        am.Type,
		Spec:        am.Spec,
		Output:      am.Output,
		Status:      am.Status,
		LastError:   am.LastError,
		SpecVersion: am.SpecVersion,
		CreatedAt:   am.CreatedAt,
		UpdatedAt:   am.UpdatedAt,
		CreatedBy:   am.CreatedBy,
		UpdatedBy:   am.UpdatedBy,
	}
}

type addonConfigModel struct {
	Project   string    `db:"project"`
	Type      string    `db:"type"`
	Configs   []byte    `db:"configs"`
	CreatedAt time.Time `db:"created_at"`
	UpdatedAt time.Time `db:"updated_at"`
}

func (st *Store) GetAddon(ctx context.Context, parentURN, typ string) (*addon.Addon, error) {
	ctx = withAddonAttributes(ctx, "GetAddon", tableAddons)

	query, args, err := sq.Select(addonColumns...).From(tableAddons).
		Where(sq.Eq{"parent_urn": parentURN, "type": typ}).
		Where(sq.Expr("deleted_at IS NULL")).
		PlaceholderFormat(sq.Dollar).ToSql()
	if err != nil {
		return nil, err
	}

	var rec addonModel
	if err := st.db.QueryRowxContext(ctx, query, args...).StructScan(&rec); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errors.ErrNotFound.WithMsgf("add-on '%s' does not exist on resource '%s'", typ, parentURN)
		}
		return nil, translateErr(err)
	}

	a := rec.toAddon()
	return &a, nil
}

func (st *Store) ListAddons(ctx context.Context, parentURN string) ([]addon.Addon, error) {
	ctx = withAddonAttributes(ctx, "ListAddons", tableAddons)

	query, args, err := sq.Select(addonColumns...).From(tableAddons).
		Where(sq.Eq{"parent_urn": parentURN}).
		Where(sq.Expr("deleted_at IS NULL")).
		OrderBy("type").
		PlaceholderFormat(sq.Dollar).ToSql()
	if err != nil {
		return nil, err
	}

	rows, err := st.db.QueryxContext(ctx, query, args...)
	if err != nil {
		return nil, translateErr(err)
	}
	defer rows.Close()

	addons := []addon.Addon{}
	for rows.Next() {
		var rec addonModel
		if err := rows.StructScan(&rec); err != nil {
			return nil, err
		}
		addons = append(addons, rec.toAddon())
	}
	return addons, rows.Err()
}

func (st *Store) CreateAddon(ctx context.Context, a *addon.Addon) error {
	ctx = withAddonAttributes(ctx, "CreateAddon", tableAddons)

	query, args, err := sq.Insert(tableAddons).
		Columns("urn", "parent_urn", "type", "spec", "output", "status", "last_error",
			"spec_version", "created_at", "updated_at", "created_by", "updated_by").
		Values(a.URN, a.ParentURN, a.Type, []byte(a.Spec), []byte(a.Output), a.Status, a.LastError,
			a.SpecVersion, a.CreatedAt, a.UpdatedAt, a.CreatedBy, a.UpdatedBy).
		Suffix(`RETURNING "id"`).
		PlaceholderFormat(sq.Dollar).ToSql()
	if err != nil {
		return err
	}

	if err := st.db.QueryRowxContext(ctx, query, args...).Scan(&a.ID); err != nil {
		err = translateErr(err)
		if errors.Is(err, errors.ErrConflict) {
			return errors.ErrConflict.WithMsgf("add-on '%s' already exists on resource '%s'", a.Type, a.ParentURN)
		}
		return err
	}
	return nil
}

func (st *Store) UpdateAddon(ctx context.Context, a addon.Addon, expectedVersion int64) error {
	ctx = withAddonAttributes(ctx, "UpdateAddon", tableAddons)

	res, err := sq.Update(tableAddons).
		SetMap(map[string]interface{}{
			"spec":         []byte(a.Spec),
			"output":       []byte(a.Output),
			"status":       a.Status,
			"last_error":   a.LastError,
			"spec_version": a.SpecVersion,
			"updated_at":   a.UpdatedAt,
			"updated_by":   a.UpdatedBy,
		}).
		Where(sq.Eq{"id": a.ID, "spec_version": expectedVersion}).
		Where(sq.Expr("deleted_at IS NULL")).
		PlaceholderFormat(sq.Dollar).
		RunWith(st.db).
		ExecContext(ctx)
	if err != nil {
		return translateErr(err)
	}

	affected, err := res.RowsAffected()
	if err != nil {
		return err
	} else if affected == 0 {
		return errors.ErrConflict.WithMsgf("add-on '%s' on resource '%s' was modified or deleted concurrently", a.Type, a.ParentURN)
	}
	return nil
}

func (st *Store) SoftDeleteAddon(ctx context.Context, id int64, deletedBy string) error {
	ctx = withAddonAttributes(ctx, "SoftDeleteAddon", tableAddons)

	res, err := sq.Update(tableAddons).
		Set("deleted_at", sq.Expr("current_timestamp")).
		Set("deleted_by", deletedBy).
		Where(sq.Eq{"id": id}).
		Where(sq.Expr("deleted_at IS NULL")).
		PlaceholderFormat(sq.Dollar).
		RunWith(st.db).
		ExecContext(ctx)
	if err != nil {
		return translateErr(err)
	}

	affected, err := res.RowsAffected()
	if err != nil {
		return err
	} else if affected == 0 {
		return errors.ErrNotFound.WithMsgf("add-on does not exist")
	}
	return nil
}

func (st *Store) GetAddonConfig(ctx context.Context, project, typ string) (*addon.Config, error) {
	ctx = withAddonAttributes(ctx, "GetAddonConfig", tableAddonConfigs)

	query, args, err := sq.Select("project", "type", "configs", "created_at", "updated_at").
		From(tableAddonConfigs).
		Where(sq.Eq{"project": project, "type": typ}).
		PlaceholderFormat(sq.Dollar).ToSql()
	if err != nil {
		return nil, err
	}

	var rec addonConfigModel
	if err := st.db.QueryRowxContext(ctx, query, args...).StructScan(&rec); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errors.ErrNotFound.WithMsgf("addon-config '%s' does not exist for project '%s'", typ, project)
		}
		return nil, translateErr(err)
	}

	return &addon.Config{
		Project:   rec.Project,
		Type:      rec.Type,
		Configs:   rec.Configs,
		CreatedAt: rec.CreatedAt,
		UpdatedAt: rec.UpdatedAt,
	}, nil
}

func (st *Store) PutAddonConfig(ctx context.Context, cfg addon.Config) error {
	ctx = withAddonAttributes(ctx, "PutAddonConfig", tableAddonConfigs)

	_, err := sq.Insert(tableAddonConfigs).
		Columns("project", "type", "configs", "created_at", "updated_at").
		Values(cfg.Project, cfg.Type, []byte(cfg.Configs), cfg.CreatedAt, cfg.UpdatedAt).
		Suffix("ON CONFLICT (project, type) DO UPDATE SET configs = EXCLUDED.configs, updated_at = EXCLUDED.updated_at").
		PlaceholderFormat(sq.Dollar).
		RunWith(st.db).
		ExecContext(ctx)
	return translateErr(err)
}

func (st *Store) DeleteAddonConfig(ctx context.Context, project, typ string) error {
	ctx = withAddonAttributes(ctx, "DeleteAddonConfig", tableAddonConfigs)

	res, err := sq.Delete(tableAddonConfigs).
		Where(sq.Eq{"project": project, "type": typ}).
		PlaceholderFormat(sq.Dollar).
		RunWith(st.db).
		ExecContext(ctx)
	if err != nil {
		return translateErr(err)
	}

	affected, err := res.RowsAffected()
	if err != nil {
		return err
	} else if affected == 0 {
		return errors.ErrNotFound.WithMsgf("addon-config '%s' does not exist for project '%s'", typ, project)
	}
	return nil
}

func withAddonAttributes(ctx context.Context, method, table string) context.Context {
	return otelsql.WithCustomAttributes(
		ctx,
		[]attribute.KeyValue{
			attribute.String("db.repository.method", method),
			attribute.String(string(semconv.DBSQLTableKey), table),
		}...,
	)
}
