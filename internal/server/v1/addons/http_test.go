package addons_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	gorillamux "github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/goto/entropy/core/addon"
	"github.com/goto/entropy/internal/server/v1/addons"
	"github.com/goto/entropy/pkg/errors"
)

const parentURN = "orn:entropy:firehose:team-a:orders"

func TestHandler(t *testing.T) {
	t.Parallel()

	base := "/api/v1beta1/resources/" + parentURN + "/addons"

	tests := []struct {
		name       string
		method     string
		path       string
		body       string
		userID     string
		svc        *fakeService
		wantStatus int
		wantBody   string
		check      func(t *testing.T, svc *fakeService)
	}{
		{
			name:       "List",
			method:     http.MethodGet,
			path:       base,
			svc:        &fakeService{addons: []addon.Addon{{URN: parentURN + ":logs", Type: "logs"}}},
			wantStatus: http.StatusOK,
			check: func(t *testing.T, svc *fakeService) {
				assert.Equal(t, parentURN, svc.gotParent)
			},
		},
		{
			name:       "Get",
			method:     http.MethodGet,
			path:       base + "/logs",
			svc:        &fakeService{},
			wantStatus: http.StatusOK,
			check: func(t *testing.T, svc *fakeService) {
				assert.Equal(t, parentURN, svc.gotParent)
				assert.Equal(t, "logs", svc.gotType)
			},
		},
		{
			name:       "ApplyPassesRequest",
			method:     http.MethodPut,
			path:       base + "/logs",
			body:       `{"spec":{"labels":{"a":"b"}},"spec_version":2}`,
			userID:     "alice",
			svc:        &fakeService{},
			wantStatus: http.StatusOK,
			check: func(t *testing.T, svc *fakeService) {
				assert.Equal(t, parentURN, svc.gotApply.ParentURN)
				assert.Equal(t, "logs", svc.gotApply.Type)
				assert.Equal(t, "alice", svc.gotApply.UserID)
				assert.JSONEq(t, `{"labels":{"a":"b"}}`, string(svc.gotApply.Spec))
				require.NotNil(t, svc.gotApply.SpecVersion)
				assert.Equal(t, int64(2), *svc.gotApply.SpecVersion)
			},
		},
		{
			name:       "ApplyRequiresUserID",
			method:     http.MethodPut,
			path:       base + "/logs",
			body:       `{}`,
			svc:        &fakeService{},
			wantStatus: http.StatusBadRequest,
			wantBody:   `{"message":"missing user-id header"}`,
		},
		{
			name:       "ApplyInvalidBody",
			method:     http.MethodPut,
			path:       base + "/logs",
			body:       `not json`,
			userID:     "alice",
			svc:        &fakeService{},
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "ApplyInvalid",
			method:     http.MethodPut,
			path:       base + "/logs",
			body:       `{}`,
			userID:     "alice",
			svc:        &fakeService{err: errors.ErrInvalid.WithMsgf("parent must be in STATUS_COMPLETED")},
			wantStatus: http.StatusBadRequest,
			wantBody:   `{"message":"parent must be in STATUS_COMPLETED"}`,
		},
		{
			name:       "ApplyConflict",
			method:     http.MethodPut,
			path:       base + "/logs",
			body:       `{}`,
			userID:     "alice",
			svc:        &fakeService{err: errors.ErrConflict},
			wantStatus: http.StatusConflict,
		},
		{
			name:       "GetNotFound",
			method:     http.MethodGet,
			path:       base + "/logs",
			svc:        &fakeService{err: errors.ErrNotFound},
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "InternalError",
			method:     http.MethodGet,
			path:       base,
			svc:        &fakeService{err: errors.New("db down")},
			wantStatus: http.StatusInternalServerError,
		},
		{
			name:       "Delete",
			method:     http.MethodDelete,
			path:       base + "/logs",
			userID:     "bob",
			svc:        &fakeService{},
			wantStatus: http.StatusOK,
			check: func(t *testing.T, svc *fakeService) {
				assert.Equal(t, "bob", svc.gotUser)
			},
		},
		{
			name:       "DeleteRequiresUserID",
			method:     http.MethodDelete,
			path:       base + "/logs",
			svc:        &fakeService{},
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "PutConfig",
			method:     http.MethodPut,
			path:       "/api/v1beta1/projects/team-a/addon-configs/logs",
			body:       `{"configs":{"endpoint":"https://logging.example.com"}}`,
			userID:     "alice",
			svc:        &fakeService{},
			wantStatus: http.StatusOK,
			check: func(t *testing.T, svc *fakeService) {
				assert.Equal(t, "team-a", svc.gotProject)
				assert.JSONEq(t, `{"endpoint":"https://logging.example.com"}`, string(svc.gotConfigs))
			},
		},
		{
			name:       "GetConfig",
			method:     http.MethodGet,
			path:       "/api/v1beta1/projects/team-a/addon-configs/logs",
			svc:        &fakeService{},
			wantStatus: http.StatusOK,
		},
		{
			name:       "DeleteConfigRequiresUserID",
			method:     http.MethodDelete,
			path:       "/api/v1beta1/projects/team-a/addon-configs/logs",
			svc:        &fakeService{},
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "MethodNotAllowed",
			method:     http.MethodPost,
			path:       base + "/logs",
			svc:        &fakeService{},
			wantStatus: http.StatusMethodNotAllowed,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			router := gorillamux.NewRouter()
			addons.Register(router, tt.svc)

			req := httptest.NewRequestWithContext(context.Background(), tt.method, tt.path, strings.NewReader(tt.body))
			if tt.userID != "" {
				req.Header.Set("user-id", tt.userID)
			}
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)

			assert.Equal(t, tt.wantStatus, rec.Code, rec.Body.String())
			if tt.wantBody != "" {
				assert.JSONEq(t, tt.wantBody, rec.Body.String())
			}
			if tt.check != nil {
				tt.check(t, tt.svc)
			}
		})
	}
}

type fakeService struct {
	addons []addon.Addon
	err    error

	gotParent  string
	gotType    string
	gotUser    string
	gotProject string
	gotConfigs json.RawMessage
	gotApply   addon.ApplyRequest
}

func (f *fakeService) Apply(_ context.Context, req addon.ApplyRequest) (*addon.Addon, error) {
	f.gotApply = req
	if f.err != nil {
		return nil, f.err
	}
	return &addon.Addon{URN: req.ParentURN + ":" + req.Type}, nil
}

func (f *fakeService) Get(_ context.Context, parentURN, typ string) (*addon.Addon, error) {
	f.gotParent, f.gotType = parentURN, typ
	if f.err != nil {
		return nil, f.err
	}
	return &addon.Addon{URN: parentURN + ":" + typ}, nil
}

func (f *fakeService) List(_ context.Context, parentURN string) ([]addon.Addon, error) {
	f.gotParent = parentURN
	return f.addons, f.err
}

func (f *fakeService) Delete(_ context.Context, parentURN, typ, deletedBy string) error {
	f.gotParent, f.gotType, f.gotUser = parentURN, typ, deletedBy
	return f.err
}

func (f *fakeService) GetConfig(_ context.Context, project, typ string) (*addon.Config, error) {
	f.gotProject = project
	if f.err != nil {
		return nil, f.err
	}
	return &addon.Config{Project: project, Type: typ, Configs: json.RawMessage(`{}`)}, nil
}

func (f *fakeService) PutConfig(_ context.Context, project, typ string, configs json.RawMessage) (*addon.Config, error) {
	f.gotProject, f.gotConfigs = project, configs
	if f.err != nil {
		return nil, f.err
	}
	return &addon.Config{Project: project, Type: typ, Configs: configs}, nil
}

func (f *fakeService) DeleteConfig(_ context.Context, project, _ string) error {
	f.gotProject = project
	return f.err
}
