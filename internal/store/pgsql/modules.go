package pgsql

import (
	"context"

	"github.com/goto/entropy/core/module"
	"github.com/goto/entropy/internal/store/pgsql/queries"
)

func (st *Store) GetModule(ctx context.Context, urn string) (*module.Module, error) {
	mod, err := st.qu.GetModuleByURN(ctx, urn)
	if err != nil {
		return nil, translateSQLErr(err)
	}

	return &module.Module{
		URN:       mod.Urn,
		Name:      mod.Name,
		Project:   mod.Project,
		Configs:   mod.Configs,
		CreatedAt: mod.CreatedAt.Time,
		UpdatedAt: mod.UpdatedAt.Time,
	}, nil
}

func (st *Store) ListModules(ctx context.Context, project string) ([]module.Module, error) {
	mods, err := st.qu.ListAllModulesForProject(ctx, project)
	if err != nil {
		return nil, translateSQLErr(err)
	}

	var modules []module.Module
	for _, mod := range mods {
		modules = append(modules, module.Module{
			URN:       mod.Urn,
			Name:      mod.Name,
			Project:   mod.Project,
			Configs:   mod.Configs,
			CreatedAt: mod.CreatedAt.Time,
			UpdatedAt: mod.UpdatedAt.Time,
		})
	}

	return modules, nil
}

func (st *Store) CreateModule(ctx context.Context, m module.Module) error {
	params := queries.InsertModuleParams{
		Urn:     m.URN,
		Project: m.Project,
		Name:    m.Name,
		Configs: m.Configs,
	}

	if err := st.qu.InsertModule(ctx, params); err != nil {
		return translateSQLErr(err)
	}
	return nil
}

func (st *Store) UpdateModule(ctx context.Context, m module.Module) error {
	params := queries.UpdateModuleParams{
		Urn:     m.URN,
		Configs: m.Configs,
	}
	if err := st.qu.UpdateModule(ctx, params); err != nil {
		return translateSQLErr(err)
	}
	return nil
}

func (st *Store) DeleteModule(ctx context.Context, urn string) error {
	if err := st.qu.DeleteModule(ctx, urn); err != nil {
		return translateSQLErr(err)
	}
	return nil
}
