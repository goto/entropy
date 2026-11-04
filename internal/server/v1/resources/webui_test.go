package resources_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	gorillamux "github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

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
			resources.RegisterWebUI(router, svc)

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
