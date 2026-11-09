package resources_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	gorillamux "github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/goto/entropy/core/addon"
	"github.com/goto/entropy/internal/server/v1/mocks"
	"github.com/goto/entropy/internal/server/v1/resources"
	"github.com/goto/entropy/pkg/errors"
)

func TestWebUIHandler(t *testing.T) {
	t.Parallel()

	const base = "/api/v1beta1/resources/urn:1/webui"

	tests := []struct {
		name       string
		method     string
		path       string
		setup      func(svc *mocks.ResourceService)
		gate       resources.WebUIGate
		wantStatus int
		wantBody   string
	}{
		{
			name:       "ForwardsGetWithPrefixStripped",
			method:     http.MethodGet,
			path:       base + "/jobs/overview?x=1",
			wantStatus: http.StatusOK,
			wantBody:   "/jobs/overview?x=1",
			setup: func(svc *mocks.ResourceService) {
				svc.EXPECT().GetWebUIProxy(mock.Anything, "urn:1").Return(
					http.HandlerFunc(func(wr http.ResponseWriter, req *http.Request) {
						_, _ = wr.Write([]byte(req.URL.RequestURI()))
					}), nil)
			},
		},
		{
			name:       "RejectsMutatingMethod",
			method:     http.MethodPost,
			path:       base + "/jobs/1/cancel",
			wantStatus: http.StatusMethodNotAllowed,
			setup:      func(svc *mocks.ResourceService) {},
		},
		{
			name:       "ResourceNotFound",
			method:     http.MethodGet,
			path:       base + "/",
			wantStatus: http.StatusNotFound,
			setup: func(svc *mocks.ResourceService) {
				svc.EXPECT().GetWebUIProxy(mock.Anything, "urn:1").Return(nil, errors.ErrNotFound)
			},
		},
		{
			name:       "UnsupportedKind",
			method:     http.MethodGet,
			path:       base + "/",
			wantStatus: http.StatusNotImplemented,
			setup: func(svc *mocks.ResourceService) {
				svc.EXPECT().GetWebUIProxy(mock.Anything, "urn:1").Return(nil, errors.ErrUnsupported)
			},
		},
		{
			name:       "GateAllowsCompletedAddon",
			method:     http.MethodGet,
			path:       base + "/overview",
			gate:       fakeGate{addon: &addon.Addon{Type: addon.TypeWebUI, Status: addon.StatusCompleted}},
			wantStatus: http.StatusOK,
			wantBody:   "/overview",
			setup: func(svc *mocks.ResourceService) {
				svc.EXPECT().GetWebUIProxy(mock.Anything, "urn:1").Return(
					http.HandlerFunc(func(wr http.ResponseWriter, req *http.Request) {
						_, _ = wr.Write([]byte(req.URL.RequestURI()))
					}), nil)
			},
		},
		{
			name:       "GateRejectsMissingAddon",
			method:     http.MethodGet,
			path:       base + "/",
			gate:       fakeGate{err: errors.ErrNotFound},
			wantStatus: http.StatusNotFound,
			wantBody:   "webui add-on is not enabled for resource 'urn:1'\n",
			setup:      func(svc *mocks.ResourceService) {},
		},
		{
			name:       "GateRejectsFailedAddon",
			method:     http.MethodGet,
			path:       base + "/",
			gate:       fakeGate{addon: &addon.Addon{Type: addon.TypeWebUI, Status: addon.StatusError}},
			wantStatus: http.StatusNotFound,
			setup:      func(svc *mocks.ResourceService) {},
		},
		{
			name:       "GateError",
			method:     http.MethodGet,
			path:       base + "/",
			gate:       fakeGate{err: errors.ErrInternal},
			wantStatus: http.StatusInternalServerError,
			setup:      func(svc *mocks.ResourceService) {},
		},
		{
			name:       "RedirectsWithoutTrailingSlash",
			method:     http.MethodGet,
			path:       base,
			wantStatus: http.StatusMovedPermanently,
			setup:      func(svc *mocks.ResourceService) {},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			svc := &mocks.ResourceService{}
			tt.setup(svc)
			router := gorillamux.NewRouter()
			resources.RegisterWebUI(router, svc, tt.gate)

			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, httptest.NewRequestWithContext(context.Background(), tt.method, tt.path, nil))

			assert.Equal(t, tt.wantStatus, rec.Code)
			if tt.wantBody != "" {
				assert.Equal(t, tt.wantBody, rec.Body.String())
			}
			svc.AssertExpectations(t)
		})
	}
}

type fakeGate struct {
	addon *addon.Addon
	err   error
}

func (f fakeGate) Get(_ context.Context, parentURN, typ string) (*addon.Addon, error) {
	if parentURN != "urn:1" || typ != addon.TypeWebUI {
		return nil, errors.ErrNotFound
	}
	return f.addon, f.err
}
