package addon

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// Add-on types known to Entropy.
const (
	TypeLogs  = "logs"
	TypeWebUI = "webui"
)

// Status of the last apply of an add-on.
const (
	StatusCompleted = "COMPLETED"
	StatusError     = "ERROR"
)

// Addon is an optional supporting resource attached to a parent resource. It is
// mutated only through the add-on API (and soft-deleted with its parent).
type Addon struct {
	ID          int64           `json:"-"`
	URN         string          `json:"urn"`
	ParentURN   string          `json:"parent_urn"`
	Type        string          `json:"type"`
	Spec        json.RawMessage `json:"spec"`
	Output      json.RawMessage `json:"output"`
	Status      string          `json:"status"`
	LastError   string          `json:"last_error"`
	SpecVersion int64           `json:"spec_version"`
	CreatedAt   time.Time       `json:"created_at"`
	UpdatedAt   time.Time       `json:"updated_at"`
	CreatedBy   string          `json:"created_by"`
	UpdatedBy   string          `json:"updated_by"`
}

// Config holds the project-level backend configuration (endpoint, credentials)
// for an add-on type.
type Config struct {
	Project   string          `json:"project"`
	Type      string          `json:"type"`
	Configs   json.RawMessage `json:"configs"`
	CreatedAt time.Time       `json:"created_at"`
	UpdatedAt time.Time       `json:"updated_at"`
}

// Store persists add-ons and add-on configs. All add-on reads return live
// (not soft-deleted) rows only.
type Store interface {
	GetAddon(ctx context.Context, parentURN, typ string) (*Addon, error)
	ListAddons(ctx context.Context, parentURN string) ([]Addon, error)
	// CreateAddon inserts a new live add-on and sets its ID.
	CreateAddon(ctx context.Context, a *Addon) error
	// UpdateAddon updates the live add-on with a.ID only if its stored
	// spec_version still equals expectedVersion, else returns ErrConflict.
	UpdateAddon(ctx context.Context, a Addon, expectedVersion int64) error
	SoftDeleteAddon(ctx context.Context, id int64, deletedBy string) error

	GetAddonConfig(ctx context.Context, project, typ string) (*Config, error)
	PutAddonConfig(ctx context.Context, cfg Config) error
	DeleteAddonConfig(ctx context.Context, project, typ string) error
}

// GenerateURN returns the URN of the add-on of the given type on parentURN.
func GenerateURN(parentURN, typ string) string {
	return fmt.Sprintf("%s:%s", parentURN, typ)
}
