package addons

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	gorillamux "github.com/gorilla/mux"
	"go.uber.org/zap"

	"github.com/goto/entropy/core/addon"
	"github.com/goto/entropy/pkg/errors"
)

const (
	headerUserID = "user-id"

	addonsPath       = "/api/v1beta1/resources/{urn}/addons"
	addonPath        = addonsPath + "/{type}"
	addonConfigsPath = "/api/v1beta1/projects/{project}/addon-configs/{type}"

	// ConfigsRouteName names the addon-config route so middlewares can skip
	// logging bodies that carry credentials.
	ConfigsRouteName = "addon-configs"
)

type AddonService interface {
	Apply(ctx context.Context, req addon.ApplyRequest) (*addon.Addon, error)
	Get(ctx context.Context, parentURN, typ string) (*addon.Addon, error)
	List(ctx context.Context, parentURN string) ([]addon.Addon, error)
	Delete(ctx context.Context, parentURN, typ, deletedBy string) error

	GetConfig(ctx context.Context, project, typ string) (*addon.Config, error)
	PutConfig(ctx context.Context, project, typ string, configs json.RawMessage) (*addon.Config, error)
	DeleteConfig(ctx context.Context, project, typ string) error
}

type applyRequest struct {
	Spec        json.RawMessage `json:"spec"`
	SpecVersion *int64          `json:"spec_version"`
}

type putConfigRequest struct {
	Configs json.RawMessage `json:"configs"`
}

type listResponse struct {
	Addons []addon.Addon `json:"addons"`
}

type errorResponse struct {
	Message string `json:"message"`
}

// Register mounts the add-on and addon-config routes on router. It must be
// called before the catch-all gRPC-gateway route.
func Register(router *gorillamux.Router, svc AddonService) {
	h := &handler{svc: svc}

	router.Path(addonsPath).Methods(http.MethodGet).HandlerFunc(h.list)
	router.Path(addonPath).Methods(http.MethodGet).HandlerFunc(h.get)
	router.Path(addonPath).Methods(http.MethodPut).HandlerFunc(h.apply)
	router.Path(addonPath).Methods(http.MethodDelete).HandlerFunc(h.delete)

	router.Name(ConfigsRouteName).Path(addonConfigsPath).
		Methods(http.MethodGet, http.MethodPut, http.MethodDelete).
		HandlerFunc(h.configs)
}

type handler struct {
	svc AddonService
}

func (h *handler) list(wr http.ResponseWriter, req *http.Request) {
	addons, err := h.svc.List(req.Context(), gorillamux.Vars(req)["urn"])
	if err != nil {
		writeError(wr, err)
		return
	}
	writeJSON(wr, http.StatusOK, listResponse{Addons: addons})
}

func (h *handler) get(wr http.ResponseWriter, req *http.Request) {
	vars := gorillamux.Vars(req)
	a, err := h.svc.Get(req.Context(), vars["urn"], vars["type"])
	if err != nil {
		writeError(wr, err)
		return
	}
	writeJSON(wr, http.StatusOK, a)
}

func (h *handler) apply(wr http.ResponseWriter, req *http.Request) {
	userID, ok := requireUserID(wr, req)
	if !ok {
		return
	}

	var body applyRequest
	if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
		writeError(wr, errors.ErrInvalid.WithMsgf("invalid request body: %s", err.Error()))
		return
	}

	vars := gorillamux.Vars(req)
	a, err := h.svc.Apply(req.Context(), addon.ApplyRequest{
		ParentURN:   vars["urn"],
		Type:        vars["type"],
		Spec:        body.Spec,
		SpecVersion: body.SpecVersion,
		UserID:      userID,
	})
	if err != nil {
		writeError(wr, err)
		return
	}
	writeJSON(wr, http.StatusOK, a)
}

func (h *handler) delete(wr http.ResponseWriter, req *http.Request) {
	userID, ok := requireUserID(wr, req)
	if !ok {
		return
	}

	vars := gorillamux.Vars(req)
	if err := h.svc.Delete(req.Context(), vars["urn"], vars["type"], userID); err != nil {
		writeError(wr, err)
		return
	}
	writeJSON(wr, http.StatusOK, struct{}{})
}

func (h *handler) configs(wr http.ResponseWriter, req *http.Request) {
	switch req.Method {
	case http.MethodPut:
		h.putConfig(wr, req)
	case http.MethodDelete:
		h.deleteConfig(wr, req)
	default:
		h.getConfig(wr, req)
	}
}

func (h *handler) getConfig(wr http.ResponseWriter, req *http.Request) {
	vars := gorillamux.Vars(req)
	cfg, err := h.svc.GetConfig(req.Context(), vars["project"], vars["type"])
	if err != nil {
		writeError(wr, err)
		return
	}
	writeJSON(wr, http.StatusOK, cfg)
}

func (h *handler) putConfig(wr http.ResponseWriter, req *http.Request) {
	if _, ok := requireUserID(wr, req); !ok {
		return
	}

	var body putConfigRequest
	if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
		writeError(wr, errors.ErrInvalid.WithMsgf("invalid request body: %s", err.Error()))
		return
	}

	vars := gorillamux.Vars(req)
	cfg, err := h.svc.PutConfig(req.Context(), vars["project"], vars["type"], body.Configs)
	if err != nil {
		writeError(wr, err)
		return
	}
	writeJSON(wr, http.StatusOK, cfg)
}

func (h *handler) deleteConfig(wr http.ResponseWriter, req *http.Request) {
	if _, ok := requireUserID(wr, req); !ok {
		return
	}

	vars := gorillamux.Vars(req)
	if err := h.svc.DeleteConfig(req.Context(), vars["project"], vars["type"]); err != nil {
		writeError(wr, err)
		return
	}
	writeJSON(wr, http.StatusOK, struct{}{})
}

func requireUserID(wr http.ResponseWriter, req *http.Request) (string, bool) {
	userID := strings.TrimSpace(req.Header.Get(headerUserID))
	if userID == "" {
		writeError(wr, errors.ErrInvalid.WithMsgf("missing %s header", headerUserID))
		return "", false
	}
	return userID, true
}

func writeError(wr http.ResponseWriter, err error) {
	e := errors.E(err)
	status := httpStatus(e)
	if status >= http.StatusInternalServerError {
		zap.L().Error("add-on request failed", zap.Error(err))
	}

	msg := e.Message
	if msg == "" {
		msg = e.Cause
	}
	writeJSON(wr, status, errorResponse{Message: msg})
}

func httpStatus(err errors.Error) int {
	switch {
	case errors.Is(err, errors.ErrInvalid):
		return http.StatusBadRequest
	case errors.Is(err, errors.ErrNotFound):
		return http.StatusNotFound
	case errors.Is(err, errors.ErrConflict):
		return http.StatusConflict
	case errors.Is(err, errors.ErrUnsupported):
		return http.StatusNotImplemented
	default:
		return http.StatusInternalServerError
	}
}

func writeJSON(wr http.ResponseWriter, status int, v any) {
	wr.Header().Set("Content-Type", "application/json")
	wr.WriteHeader(status)
	_ = json.NewEncoder(wr).Encode(v)
}
