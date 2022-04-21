package resource

//go:generate mockery --name=Repository -r --case underscore --with-expecter --structname ResourceRepository --filename=resource_repository.go --output=./mocks

import (
	"context"
	"strings"
	"time"

	"github.com/odpf/entropy/pkg/errors"
)

const (
	StatusUnspecified Status = "STATUS_UNSPECIFIED"
	StatusPending     Status = "STATUS_PENDING"
	StatusError       Status = "STATUS_ERROR"
	StatusRunning     Status = "STATUS_RUNNING"
	StatusStopped     Status = "STATUS_STOPPED"
	StatusCompleted   Status = "STATUS_COMPLETED"
)

type Repository interface {
	Migrate(ctx context.Context) error

	GetByURN(ctx context.Context, urn string) (*Resource, error)
	List(ctx context.Context, filter map[string]string) ([]*Resource, error)
	Create(ctx context.Context, r Resource) error
	Update(ctx context.Context, r Resource) error
	Delete(ctx context.Context, urn string) error
}

type Resource struct {
	URN       string                 `bson:"urn"`
	Kind      string                 `bson:"kind"`
	Name      string                 `bson:"name"`
	Parent    string                 `bson:"parent"`
	Status    Status                 `bson:"status"`
	Labels    map[string]string      `bson:"labels"`
	Configs   map[string]interface{} `bson:"configs"`
	Providers []ProviderSelector     `bson:"providers"`
	CreatedAt time.Time              `bson:"created_at"`
	UpdatedAt time.Time              `bson:"updated_at"`
}

type Action struct {
	Name   string
	Params map[string]interface{}
}

type Status string

type Updates struct {
	Configs map[string]interface{}
}

type ProviderSelector struct {
	URN    string `bson:"urn"`
	Target string `bson:"target"`
}

func (res *Resource) Validate() error {
	res.Kind = strings.TrimSpace(res.Kind)
	res.Name = strings.TrimSpace(res.Name)
	res.Parent = strings.TrimSpace(res.Parent)
	res.Status = Status(strings.TrimSpace(string(res.Status)))

	if res.Kind == "" {
		return errors.ErrInvalid.WithMsgf("resource must have a kind")
	}
	if res.Name == "" {
		return errors.ErrInvalid.WithMsgf("resource must have a name")
	}
	if res.Parent == "" {
		return errors.ErrInvalid.WithMsgf("resource must have a parent")
	}

	if res.Status == "" {
		res.Status = StatusUnspecified
	}

	res.URN = generateURN(*res)
	return nil
}

func generateURN(res Resource) string {
	return strings.Join([]string{
		sanitizeString(res.Parent),
		sanitizeString(res.Name),
		sanitizeString(res.Kind),
	}, "-")
}

func sanitizeString(s string) string {
	return strings.ReplaceAll(s, " ", "_")
}
