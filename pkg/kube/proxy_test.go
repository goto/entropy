package kube

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/client-go/rest"
)

func TestClient_ProxyServiceHandler(t *testing.T) {
	t.Parallel()

	var gotPath, gotAuth string
	apiServer := httptest.NewServer(http.HandlerFunc(func(wr http.ResponseWriter, req *http.Request) {
		gotPath = req.URL.RequestURI()
		gotAuth = req.Header.Get("Authorization")
		_, _ = wr.Write([]byte("ok"))
	}))
	defer apiServer.Close()

	c := Client{restConfig: rest.Config{Host: apiServer.URL}}
	h, err := c.ProxyServiceHandler("ns", "http", "svc-rest", "8081")
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodGet, "/jobs/overview?a=b", nil)
	req.Header.Set("Authorization", "caller-token")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "/api/v1/namespaces/ns/services/http:svc-rest:8081/proxy/jobs/overview?a=b", gotPath)
	assert.Empty(t, gotAuth)
}
