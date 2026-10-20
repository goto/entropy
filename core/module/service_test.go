package module_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/goto/entropy/core/mocks"
	"github.com/goto/entropy/core/module"
	"github.com/goto/entropy/core/resource"
	"github.com/goto/entropy/pkg/errors"
)

func TestService_IsRunning(t *testing.T) {
	t.Parallel()

	errBoom := errors.New("boom")
	exr := module.ExpandedResource{Resource: resource.Resource{
		Kind:  "foo",
		State: resource.State{Output: []byte(`{"k":"v"}`)},
	}}

	tests := []struct {
		name    string
		reg     func() *mocks.ModuleRegistry
		want    bool
		wantErr error
	}{
		{
			name: "UnknownKind",
			reg: func() *mocks.ModuleRegistry {
				r := &mocks.ModuleRegistry{}
				r.EXPECT().GetDescriptor("foo").Return(module.Descriptor{}, errors.ErrNotFound).Once()
				return r
			},
			wantErr: errors.ErrInvalid,
		},
		{
			name: "RegistryError",
			reg: func() *mocks.ModuleRegistry {
				r := &mocks.ModuleRegistry{}
				r.EXPECT().GetDescriptor("foo").Return(module.Descriptor{}, errBoom).Once()
				return r
			},
			wantErr: errBoom,
		},
		{
			name: "Unsupported",
			reg: func() *mocks.ModuleRegistry {
				r := &mocks.ModuleRegistry{}
				r.EXPECT().GetDescriptor("foo").Return(module.Descriptor{Kind: "foo"}, nil).Once()
				return r
			},
			wantErr: errors.ErrUnsupported,
		},
		{
			name: "DescriptorError",
			reg: func() *mocks.ModuleRegistry {
				r := &mocks.ModuleRegistry{}
				r.EXPECT().GetDescriptor("foo").Return(module.Descriptor{
					Kind:      "foo",
					IsRunning: func(resource.Resource) (bool, error) { return false, errBoom },
				}, nil).Once()
				return r
			},
			wantErr: errBoom,
		},
		{
			name: "Running",
			reg: func() *mocks.ModuleRegistry {
				r := &mocks.ModuleRegistry{}
				r.EXPECT().GetDescriptor("foo").Return(module.Descriptor{
					Kind: "foo",
					IsRunning: func(res resource.Resource) (bool, error) {
						return string(res.State.Output) == `{"k":"v"}`, nil
					},
				}, nil).Once()
				return r
			},
			want: true,
		},
		{
			name: "NotRunning",
			reg: func() *mocks.ModuleRegistry {
				r := &mocks.ModuleRegistry{}
				r.EXPECT().GetDescriptor("foo").Return(module.Descriptor{
					Kind:      "foo",
					IsRunning: func(resource.Resource) (bool, error) { return false, nil },
				}, nil).Once()
				return r
			},
			want: false,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			reg := tt.reg()
			svc := module.NewService(reg, &mocks.ModuleStore{})

			got, err := svc.IsRunning(context.Background(), exr)
			if tt.wantErr != nil {
				assert.Error(t, err)
				assert.True(t, errors.Is(err, tt.wantErr))
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, tt.want, got)

			// driver must never be initialised for the running check.
			reg.AssertNotCalled(t, "GetDriver", mock.Anything, mock.Anything)
		})
	}
}
