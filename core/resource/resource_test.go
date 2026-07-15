package resource_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/goto/entropy/core/resource"
	"github.com/goto/entropy/pkg/errors"
)

func TestResource_Validate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		res  resource.Resource
		want error
	}{
		{
			name: "InvalidName",
			res: resource.Resource{
				Kind:    "fake",
				Name:    "",
				Project: "bar",
			},
			want: errors.ErrInvalid,
		},
		{
			name: "InvalidKind",
			res: resource.Resource{
				Kind:    "",
				Name:    "foo",
				Project: "bar",
			},
			want: errors.ErrInvalid,
		},
		{
			name: "InvalidProject",
			res: resource.Resource{
				Kind:    "fake",
				Name:    "foo",
				Project: "978997",
			},
			want: errors.ErrInvalid,
		},
		{
			name: "ValidResourceWithNameStartingWithANumber",
			res: resource.Resource{
				Kind:    "fake",
				Name:    "12a1lpha",
				Project: "goto",
			},
			want: nil,
		},
		{
			name: "ValidResourceWithNameAsNumber",
			res: resource.Resource{
				Kind:    "fake",
				Name:    "112233",
				Project: "goto",
			},
			want: nil,
		},
		{
			name: "ValidResource",
			res: resource.Resource{
				Kind:    "fake",
				Name:    "foo",
				Project: "goto",
			},
			want: nil,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := tt.res.Validate(true)
			assert.Truef(t, errors.Is(got, tt.want), "want=%v, got=%v", tt.want, got)
		})
	}
}

func TestFilter_Apply_StateOutput(t *testing.T) {
	t.Parallel()

	stateOutput := []byte(`{
		"deployment": {"name": "gjk-firehose", "replicas": 12},
		"desired_status": "RUNNING",
		"pods": [
			{"name": "gjk-firehose-584525rgp", "status": "Running"},
			{"name": "gjk-firehose-58456hj6n", "status": "Running"}
		]
	}`)

	res := resource.Resource{
		Kind:    "firehose",
		Name:    "insurance",
		Project: "gjk-p-acc",
		State:   resource.State{Output: stateOutput},
	}

	tests := []struct {
		name        string
		stateOutput map[string]string
		wantMatch   bool
	}{
		{
			name:        "ArrayElementNameMatch",
			stateOutput: map[string]string{"pods.name": "gjk-firehose-584525rgp"},
			wantMatch:   true,
		},
		{
			name:        "ArrayElementNameNoMatch",
			stateOutput: map[string]string{"pods.name": "does-not-exist"},
			wantMatch:   false,
		},
		{
			name:        "NestedScalarMatch",
			stateOutput: map[string]string{"desired_status": "RUNNING"},
			wantMatch:   true,
		},
		{
			name:        "NestedObjectFieldMatch",
			stateOutput: map[string]string{"deployment.name": "gjk-firehose"},
			wantMatch:   true,
		},
		{
			name:        "NumberLeafMatch",
			stateOutput: map[string]string{"deployment.replicas": "12"},
			wantMatch:   true,
		},
		{
			name:        "MissingKeyNoMatch",
			stateOutput: map[string]string{"missing.field": "x"},
			wantMatch:   false,
		},
		{
			name: "MultipleEntriesAndSemanticsFail",
			stateOutput: map[string]string{
				"desired_status": "RUNNING",
				"pods.name":      "does-not-exist",
			},
			wantMatch: false,
		},
		{
			name: "MultipleEntriesAndSemanticsPass",
			stateOutput: map[string]string{
				"desired_status": "RUNNING",
				"pods.status":    "Running",
			},
			wantMatch: true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			f := resource.Filter{StateOutput: tt.stateOutput}
			got := f.Apply([]resource.Resource{res})
			if tt.wantMatch {
				assert.Len(t, got, 1)
			} else {
				assert.Empty(t, got)
			}
		})
	}
}

func TestFilter_Apply_StateOutput_EmptyOrInvalid(t *testing.T) {
	t.Parallel()

	f := resource.Filter{StateOutput: map[string]string{"pods.name": "x"}}

	empty := resource.Resource{Kind: "firehose"}
	assert.Empty(t, f.Apply([]resource.Resource{empty}), "empty state.output should not match")

	invalid := resource.Resource{Kind: "firehose", State: resource.State{Output: []byte(`{not-json`)}}
	assert.Empty(t, f.Apply([]resource.Resource{invalid}), "invalid state.output should not match")
}
