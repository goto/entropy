package resource

//go:generate mockery --name=Store -r --case underscore --with-expecter --structname ResourceStore --filename=resource_store.go --output=../mocks

import (
	"context"
	"encoding/json"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/goto/entropy/pkg/errors"
)

const urnSeparator = ":"

var namingPattern = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_-]+$`)
var namingPatternStartingWithDigits = regexp.MustCompile(`^\d*[A-Za-z0-9_-]+$`)

type Store interface {
	GetByURN(ctx context.Context, urn string) (*Resource, error)
	List(ctx context.Context, filter Filter, withSpecConfigs bool) ([]Resource, error)

	Create(ctx context.Context, r Resource, hooks ...MutationHook) error
	Update(ctx context.Context, r Resource, saveRevision bool, reason string, hooks ...MutationHook) error
	Delete(ctx context.Context, urn string, hooks ...MutationHook) error

	Revisions(ctx context.Context, selector RevisionsSelector) ([]Revision, error)

	SyncOne(ctx context.Context, scope map[string][]string, syncFn SyncFn) error
}

type SyncFn func(ctx context.Context, res Resource) (*Resource, error)

// MutationHook values are passed to mutation operations of resource storage
// to handle any transactional requirements.
type MutationHook func(ctx context.Context) error

type PendingHandler func(ctx context.Context, res Resource) (*Resource, bool, error)

type Resource struct {
	URN       string            `json:"urn"`
	Kind      string            `json:"kind"`
	Name      string            `json:"name"`
	Project   string            `json:"project"`
	Labels    map[string]string `json:"labels"`
	CreatedAt time.Time         `json:"created_at"`
	UpdatedAt time.Time         `json:"updated_at"`
	UpdatedBy string            `json:"updated_by"`
	CreatedBy string            `json:"created_by"`
	Spec      Spec              `json:"spec"`
	State     State             `json:"state"`
}

type PagedResource struct {
	Count     int32
	Resources []Resource
}

type Spec struct {
	Configs      json.RawMessage   `json:"configs"`
	Dependencies map[string]string `json:"dependencies"`
}

type Filter struct {
	Kind        string            `json:"kind"`
	Project     string            `json:"project"`
	Labels      map[string]string `json:"labels"`
	StateOutput map[string]string `json:"state_output"` // dot-path into state.output -> exact value
	PageSize    int32             `json:"page_size"`
	PageNum     int32             `json:"page_num"`
}

type UpdateRequest struct {
	Spec   Spec              `json:"spec"`
	Labels map[string]string `json:"labels"`
	UserID string
}

type RevisionsSelector struct {
	URN string `json:"urn"`
}

type Revision struct {
	ID        int64             `json:"id"`
	URN       string            `json:"urn"`
	Reason    string            `json:"reason"`
	Labels    map[string]string `json:"labels"`
	CreatedAt time.Time         `json:"created_at"`
	CreatedBy string            `json:"created_by"`

	Spec Spec `json:"spec"`
}

func (res *Resource) Validate(isCreate bool) error {
	res.Kind = strings.TrimSpace(res.Kind)
	res.Name = strings.TrimSpace(res.Name)
	res.Project = strings.TrimSpace(res.Project)

	if !namingPattern.MatchString(res.Kind) {
		return errors.ErrInvalid.WithMsgf("kind must match pattern '%s'", namingPattern)
	}
	if !namingPatternStartingWithDigits.MatchString(res.Name) {
		return errors.ErrInvalid.WithMsgf("name must match pattern '%s'", namingPatternStartingWithDigits)
	}
	if !namingPattern.MatchString(res.Project) {
		return errors.ErrInvalid.WithMsgf("project must match pattern '%s'", namingPattern)
	}

	if res.State.Status == "" {
		res.State.Status = StatusUnspecified
	}

	if isCreate {
		res.URN = GenerateURN(res.Kind, res.Project, res.Name)
	}
	return nil
}

func (f Filter) Apply(arr []Resource) []Resource {
	var res []Resource
	for _, r := range arr {
		if f.isMatch(r) {
			res = append(res, r)
		}
	}
	return res
}

func (f Filter) isMatch(r Resource) bool {
	kindMatch := f.Kind == "" || f.Kind == r.Kind
	projectMatch := f.Project == "" || f.Project == r.Project
	if !kindMatch || !projectMatch {
		return false
	}

	for k, v := range f.Labels {
		if r.Labels[k] != v {
			return false
		}
	}

	for path, want := range f.StateOutput {
		if !matchStateOutput(r.State.Output, path, want) {
			return false
		}
	}

	return true
}

// matchStateOutput reports whether the nested state.output JSON has a leaf,
// reachable by the dot-separated path, whose stringified value equals want.
// When a node along the path is an array, the remaining path is applied to
// each element and the match succeeds if any element matches.
func matchStateOutput(raw json.RawMessage, path, want string) bool {
	if len(raw) == 0 {
		return false
	}

	var decoded interface{}
	if err := json.Unmarshal(raw, &decoded); err != nil {
		return false
	}

	return matchPath(decoded, strings.Split(path, "."), want)
}

func matchPath(node interface{}, segments []string, want string) bool {
	// Descend into arrays: match if any element matches the remaining path.
	if arr, ok := node.([]interface{}); ok {
		for _, elem := range arr {
			if matchPath(elem, segments, want) {
				return true
			}
		}
		return false
	}

	if len(segments) == 0 {
		return leafEquals(node, want)
	}

	obj, ok := node.(map[string]interface{})
	if !ok {
		return false
	}

	next, ok := obj[segments[0]]
	if !ok {
		return false
	}

	return matchPath(next, segments[1:], want)
}

func leafEquals(node interface{}, want string) bool {
	switch v := node.(type) {
	case string:
		return v == want
	case bool:
		return strconv.FormatBool(v) == want
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64) == want
	default:
		return false
	}
}

// GenerateURN generates an Entropy URN address for the given combination.
// Note: Changing this will invalidate all existing resource identifiers.
func GenerateURN(kind, project, name string) string {
	parts := []string{"orn", "entropy", kind, project, name}
	return strings.Join(parts, urnSeparator)
}
