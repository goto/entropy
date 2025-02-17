package core

import (
	"bytes"
	"context"

	"github.com/goto/entropy/core/module"
	"github.com/goto/entropy/core/resource"
	"github.com/goto/entropy/pkg/errors"
)

func (svc *Service) GetResource(ctx context.Context, urn string) (*resource.Resource, error) {
	res, err := svc.store.GetByURN(ctx, urn)
	if err != nil {
		if errors.Is(err, errors.ErrNotFound) {
			return nil, errors.ErrNotFound.WithMsgf("resource with urn '%s' not found", urn)
		}
		return nil, errors.ErrInternal.WithCausef(err.Error())
	}

	modSpec, err := svc.generateModuleSpec(ctx, *res)
	if err != nil {
		return nil, err
	}

	output, err := svc.moduleSvc.GetOutput(ctx, *modSpec)
	if err != nil {
		return nil, err
	}

	if !bytes.Equal(res.State.Output, output) {
		res.State.Output = output
		err = svc.store.Update(ctx, *res, false, "")
		if err != nil {
			return nil, err
		}
	}

	*res, err = svc.moduleSvc.MaskSecrets(ctx, *res)
	if err != nil {
		return nil, err
	}

	return res, nil
}

func (svc *Service) ListResources(ctx context.Context, filter resource.Filter, withSpecConfigs bool) (resource.PagedResource, error) {
	resources, err := svc.store.List(ctx, filter, withSpecConfigs)
	if err != nil {
		return resource.PagedResource{}, errors.ErrInternal.WithCausef(err.Error())
	}

	resources = filter.Apply(resources)
	return resource.PagedResource{
		Count:     int32(len(resources)),
		Resources: resources,
	}, nil
}

func (svc *Service) GetLog(ctx context.Context, urn string, filter map[string]string) (<-chan module.LogChunk, error) {
	res, err := svc.GetResource(ctx, urn)
	if err != nil {
		return nil, err
	}

	modSpec, err := svc.generateModuleSpec(ctx, *res)
	if err != nil {
		return nil, err
	}

	logCh, err := svc.moduleSvc.StreamLogs(ctx, *modSpec, filter)
	if err != nil {
		if errors.Is(err, errors.ErrUnsupported) {
			return nil, errors.ErrUnsupported.WithMsgf("log streaming not supported for kind '%s'", res.Kind)
		}
		return nil, err
	}
	return logCh, nil
}

func (svc *Service) GetRevisions(ctx context.Context, selector resource.RevisionsSelector) ([]resource.Revision, error) {
	revs, err := svc.store.Revisions(ctx, selector)
	if err != nil {
		return nil, errors.ErrInternal.WithCausef(err.Error())
	}
	return revs, nil
}
