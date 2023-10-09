package pgsql

import (
	"context"
	"encoding/json"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/goto/entropy/core/resource"
	"github.com/goto/entropy/internal/store/pgsql/queries"
)

func (st *Store) GetByURN(ctx context.Context, urn string) (*resource.Resource, error) {
	res, err := st.qu.GetResourceByURN(ctx, urn)
	if err != nil {
		return nil, translateSQLErr(err)
	}

	var nextSyncAt *time.Time
	if res.StateNextSync.Valid {
		nextSyncAt = &res.StateNextSync.Time
	}

	var syncResult resource.SyncResult
	if len(res.StateSyncResult) > 0 {
		if err := json.Unmarshal(res.StateSyncResult, &syncResult); err != nil {
			return nil, err
		}
	}

	var deps map[string]string
	if len(res.Dependencies) > 0 {
		if err := json.Unmarshal(res.Dependencies, &deps); err != nil {
			return nil, err
		}
	}

	return &resource.Resource{
		URN:       res.Urn,
		Kind:      res.Kind,
		Name:      res.Name,
		Project:   res.Project,
		Labels:    tagsToLabelMap(res.Tags),
		CreatedAt: res.CreatedAt.Time,
		UpdatedAt: res.UpdatedAt.Time,
		UpdatedBy: res.UpdatedBy,
		CreatedBy: res.CreatedBy,
		Spec: resource.Spec{
			Configs:      res.SpecConfigs,
			Dependencies: deps,
		},
		State: resource.State{
			Status:     res.StateStatus,
			Output:     res.StateOutput,
			ModuleData: res.StateModuleData,
			NextSyncAt: nextSyncAt,
			SyncResult: syncResult,
		},
	}, nil
}

func (st *Store) List(ctx context.Context, filter resource.Filter) ([]resource.Resource, error) {
	params := queries.ListResourceURNsByFilterParams{
		Project: pgtype.Text{
			Valid:  filter.Project != "",
			String: filter.Project,
		},
		Kind: pgtype.Text{
			Valid:  filter.Kind != "",
			String: filter.Kind,
		},
	}

	rows, err := st.qu.ListResourceURNsByFilter(ctx, params)
	if err != nil {
		return nil, translateSQLErr(err)
	}

	var result []resource.Resource
	for _, res := range rows {
		var nextSyncAt *time.Time
		if res.StateNextSync.Valid {
			nextSyncAt = &res.StateNextSync.Time
		}

		var syncResult resource.SyncResult
		if len(res.StateSyncResult) > 0 {
			if err := json.Unmarshal(res.StateSyncResult, &syncResult); err != nil {
				return nil, err
			}
		}

		result = append(result, resource.Resource{
			URN:       res.Urn,
			Kind:      res.Kind,
			Name:      res.Name,
			Project:   res.Project,
			Labels:    tagsToLabelMap(res.Tags),
			CreatedAt: res.CreatedAt.Time,
			UpdatedAt: res.UpdatedAt.Time,
			UpdatedBy: res.UpdatedBy,
			CreatedBy: res.CreatedBy,
			Spec: resource.Spec{
				Configs:      res.SpecConfigs,
				Dependencies: nil,
			},
			State: resource.State{
				Status:     res.StateStatus,
				Output:     res.StateOutput,
				ModuleData: res.StateModuleData,
				NextSyncAt: nextSyncAt,
				SyncResult: syncResult,
			},
		})
	}

	return result, nil
}

func (st *Store) Create(ctx context.Context, r resource.Resource, hooks ...resource.MutationHook) error {
	createResource := func(ctx context.Context, tx pgx.Tx) error {
		id, err := st.qu.InsertResource(ctx, queries.InsertResourceParams{})
		if err != nil {
			return err
		}

		var params []queries.InsertResourceTagsParams
		for key, value := range r.Labels {
			params = append(params, queries.InsertResourceTagsParams{
				Tag:        labelToTag(key, value),
				ResourceID: id,
			})
		}

		if _, err := st.qu.InsertResourceTags(ctx, params); err != nil {
			return err
		}

		for key, dependsOn := range r.Spec.Dependencies {
			if err := st.qu.InsertResourceDependency(ctx, queries.InsertResourceDependencyParams{
				ResourceID:    id,
				DependencyKey: key,
				Urn:           dependsOn,
			}); err != nil {
				return err
			}
		}

		return runAllHooks(ctx, hooks)
	}

	if err := withinTx(ctx, st.pgx, false, createResource); err != nil {
		return translateSQLErr(err)
	}
	return nil
}

func (st *Store) Update(ctx context.Context, r resource.Resource, saveRevision bool, reason string, hooks ...resource.MutationHook) error {
	updateParams := queries.UpdateResourceParams{
		Urn:             r.URN,
		UpdatedBy:       r.UpdatedBy,
		SpecConfigs:     r.Spec.Configs,
		StateStatus:     r.State.Status,
		StateOutput:     r.State.Output,
		StateModuleData: r.State.ModuleData,
	}

	syncResult, err := json.Marshal(r.State.SyncResult)
	if err != nil {
		return err
	}
	updateParams.StateSyncResult = syncResult

	if r.State.NextSyncAt != nil {
		updateParams.StateNextSync = pgtype.Timestamptz{
			Time:  *r.State.NextSyncAt,
			Valid: true,
		}
	}

	createResource := func(ctx context.Context, tx pgx.Tx) error {
		if err := st.qu.DeleteResourceTagsByURN(ctx, r.URN); err != nil {
			return err
		}

		if err := st.qu.DeleteResourceDependenciesByURN(ctx, r.URN); err != nil {
			return err
		}

		resourceID, err := st.qu.UpdateResource(ctx, updateParams)
		if err != nil {
			return err
		}

		var params []queries.InsertResourceTagsParams
		for key, value := range r.Labels {
			params = append(params, queries.InsertResourceTagsParams{
				Tag:        labelToTag(key, value),
				ResourceID: resourceID,
			})
		}

		if _, err := st.qu.InsertResourceTags(ctx, params); err != nil {
			return err
		}

		for key, dependsOn := range r.Spec.Dependencies {
			if err := st.qu.InsertResourceDependency(ctx, queries.InsertResourceDependencyParams{
				Urn:           dependsOn,
				ResourceID:    resourceID,
				DependencyKey: key,
			}); err != nil {
				return err
			}
		}

		if saveRevision {
			revisionParams := queries.InsertRevisionParams{
				ResourceID:  resourceID,
				Reason:      reason,
				SpecConfigs: r.Spec.Configs,
				CreatedBy:   r.UpdatedBy,
			}
			if err := st.qu.InsertRevision(ctx, revisionParams); err != nil {
				return translateSQLErr(err)
			}
		}

		return runAllHooks(ctx, hooks)
	}

	if err := withinTx(ctx, st.pgx, false, createResource); err != nil {
		return translateSQLErr(err)
	}
	return nil
}

func (st *Store) Delete(ctx context.Context, urn string, hooks ...resource.MutationHook) error {
	deleteResource := func(ctx context.Context, tx pgx.Tx) error {
		txQueries := st.qu.WithTx(tx)
		if err := txQueries.DeleteResourceTagsByURN(ctx, urn); err != nil {
			return err
		}
		if err := txQueries.DeleteResourceDependenciesByURN(ctx, urn); err != nil {
			return err
		}
		if err := txQueries.DeleteResourceByURN(ctx, urn); err != nil {
			return err
		}
		return runAllHooks(ctx, hooks)
	}

	if err := withinTx(ctx, st.pgx, false, deleteResource); err != nil {
		return translateSQLErr(err)
	}
	return nil
}

func (st *Store) Revisions(ctx context.Context, selector resource.RevisionsSelector) ([]resource.Revision, error) {
	revs, err := st.qu.ListResourceRevisions(ctx, selector.URN)
	if err != nil {
		return nil, translateSQLErr(err)
	}

	depsJSON, err := st.qu.GetResourceDependencies(ctx, selector.URN)
	if err != nil {
		return nil, translateSQLErr(err)
	}

	var deps map[string]string
	if len(depsJSON) > 0 {
		if err := json.Unmarshal(depsJSON, &deps); err != nil {
			return nil, err
		}
	}

	var result []resource.Revision
	for _, rev := range revs {
		result = append(result, resource.Revision{
			ID:        rev.ID,
			URN:       selector.URN,
			Reason:    rev.Reason,
			Labels:    tagsToLabelMap(rev.Tags),
			CreatedAt: rev.CreatedAt.Time,
			CreatedBy: rev.CreatedBy,
			Spec: resource.Spec{
				Configs:      rev.SpecConfigs,
				Dependencies: deps,
			},
		})
	}

	return result, nil
}

func (st *Store) SyncOne(ctx context.Context, syncFn resource.SyncFn) error {
	// TODO implement me
	panic("implement me")
}
