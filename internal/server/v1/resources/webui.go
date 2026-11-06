package resources

import (
	"context"
	"net/http"

	gorillamux "github.com/gorilla/mux"
	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"google.golang.org/grpc/status"

	"github.com/goto/entropy/core/addon"
	"github.com/goto/entropy/internal/server/serverutils"
	"github.com/goto/entropy/pkg/errors"
)

const (
	// WebUIPathPrefix is the path under which the web UI of a resource is served.
	WebUIPathPrefix = "/api/v1beta1/resources/{urn}/webui"
	// WebUIRouteName names the route so middlewares can identify it.
	WebUIRouteName = "resource-webui"
)

// WebUIGate looks up the add-on of a resource that enables its web UI.
type WebUIGate interface {
	Get(ctx context.Context, parentURN, typ string) (*addon.Addon, error)
}

// RegisterWebUI mounts a read-only reverse proxy to the web UI of a resource on router.
// When gate is set, the UI is served only while the webui add-on of the resource
// is enabled.
func RegisterWebUI(router *gorillamux.Router, svc ResourceService, gate WebUIGate) {
	router.Name(WebUIRouteName).PathPrefix(WebUIPathPrefix + "/").
		Handler(webUIHandler(svc, gate))
	// redirect so that relative links of the UI resolve under the prefix.
	router.Path(WebUIPathPrefix).Handler(http.RedirectHandler("webui/", http.StatusMovedPermanently))
}

func webUIHandler(svc ResourceService, gate WebUIGate) http.Handler {
	return http.HandlerFunc(func(wr http.ResponseWriter, req *http.Request) {
		if req.Method != http.MethodGet && req.Method != http.MethodHead {
			wr.Header().Set("Allow", "GET, HEAD")
			http.Error(wr, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		urn := gorillamux.Vars(req)["urn"]
		if gate != nil {
			a, err := gate.Get(req.Context(), urn, addon.TypeWebUI)
			if err != nil && !errors.Is(err, errors.ErrNotFound) {
				http.Error(wr, err.Error(), http.StatusInternalServerError)
				return
			}
			if a == nil || a.Status != addon.StatusCompleted {
				http.Error(wr, "webui add-on is not enabled for resource '"+urn+"'", http.StatusNotFound)
				return
			}
		}

		proxy, err := svc.GetWebUIProxy(req.Context(), urn)
		if err != nil {
			code := runtime.HTTPStatusFromCode(status.Code(serverutils.ToRPCError(err)))
			if errors.Is(err, errors.ErrUnsupported) {
				code = http.StatusNotImplemented
			}
			http.Error(wr, err.Error(), code)
			return
		}

		http.StripPrefix("/api/v1beta1/resources/"+urn+"/webui", proxy).ServeHTTP(wr, req)
	})
}
