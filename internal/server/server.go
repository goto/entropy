package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	gorillamux "github.com/gorilla/mux"
	grpcmiddleware "github.com/grpc-ecosystem/go-grpc-middleware"
	grpczap "github.com/grpc-ecosystem/go-grpc-middleware/logging/zap"
	grpcrecovery "github.com/grpc-ecosystem/go-grpc-middleware/recovery"
	grpcctxtags "github.com/grpc-ecosystem/go-grpc-middleware/tags"
	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"github.com/newrelic/go-agent/v3/integrations/nrgorilla"
	"github.com/newrelic/go-agent/v3/integrations/nrgrpc"
	"github.com/newrelic/go-agent/v3/newrelic"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
	"google.golang.org/protobuf/encoding/protojson"

	"github.com/goto/entropy/internal/server/serverutils"
	modulesv1 "github.com/goto/entropy/internal/server/v1/modules"
	resourcesv1 "github.com/goto/entropy/internal/server/v1/resources"
	"github.com/goto/entropy/pkg/common"
	"github.com/goto/entropy/pkg/masking"
	"github.com/goto/entropy/pkg/version"
	commonv1 "github.com/goto/entropy/proto/gotocompany/common/v1"
	entropyv1beta1 "github.com/goto/entropy/proto/gotocompany/entropy/v1beta1"
	"github.com/goto/salt/mux"
)

const (
	gracePeriod    = 5 * time.Second
	readTimeout    = 120 * time.Second
	writeTimeout   = 600 * time.Second
	maxHeaderBytes = 1 << 20
)

// Serve initialises all the gRPC+HTTP API routes, starts listening for requests at addr, and blocks until server exits.
// Server exits gracefully when context is cancelled.
func Serve(ctx context.Context, httpAddr, grpcAddr string, nrApp *newrelic.Application,
	resourceSvc resourcesv1.ResourceService, moduleSvc modulesv1.ModuleService, maskKey []byte,
) error {
	var masker *masking.Masker
	if len(maskKey) > 0 {
		masker = masking.New(maskKey)
	}
	modConfigLookup := moduleConfigLookup{svc: moduleSvc}

	grpcOpts := []grpc.ServerOption{
		grpc.UnaryInterceptor(grpcmiddleware.ChainUnaryServer(
			grpcrecovery.UnaryServerInterceptor(),
			grpcctxtags.UnaryServerInterceptor(),
			grpczap.UnaryServerInterceptor(zap.L()),
			nrgrpc.UnaryServerInterceptor(nrApp),
		)),
		grpc.StatsHandler(otelgrpc.NewServerHandler()),
	}
	grpcServer := grpc.NewServer(grpcOpts...)
	rpcHTTPGateway := runtime.NewServeMux(
		runtime.WithMarshalerOption(runtime.MIMEWildcard, &runtime.JSONPb{
			MarshalOptions: protojson.MarshalOptions{
				UseProtoNames:   true,
				EmitUnpopulated: true,
			},
			UnmarshalOptions: protojson.UnmarshalOptions{
				DiscardUnknown: true,
			},
		}),
		runtime.WithMetadata(serverutils.ExtractRequestMetadata),
	)

	reflection.Register(grpcServer)

	commonServiceRPC := common.New(version.GetVersionAndBuildInfo())
	grpcServer.RegisterService(&commonv1.CommonService_ServiceDesc, commonServiceRPC)
	if err := commonv1.RegisterCommonServiceHandlerServer(ctx, rpcHTTPGateway, commonServiceRPC); err != nil {
		return err
	}

	resourceServiceRPC := &resourcesv1.LogWrapper{
		ResourceServiceServer: resourcesv1.NewAPIServer(resourceSvc, masker, modConfigLookup),
	}
	grpcServer.RegisterService(&entropyv1beta1.ResourceService_ServiceDesc, resourceServiceRPC)
	if err := entropyv1beta1.RegisterResourceServiceHandlerServer(ctx, rpcHTTPGateway, resourceServiceRPC); err != nil {
		return err
	}

	moduleServiceRPC := modulesv1.NewAPIServer(moduleSvc, masker)
	grpcServer.RegisterService(&entropyv1beta1.ModuleService_ServiceDesc, moduleServiceRPC)
	if err := entropyv1beta1.RegisterModuleServiceHandlerServer(ctx, rpcHTTPGateway, moduleServiceRPC); err != nil {
		return err
	}

	httpRouter := gorillamux.NewRouter()
	httpRouter.Use(
		withOpenTelemetry(),
		nrgorilla.Middleware(nrApp),
	)
	httpRouter.PathPrefix("/api/").Handler(http.StripPrefix("/api", rpcHTTPGateway))
	httpRouter.Handle("/ping", http.HandlerFunc(func(wr http.ResponseWriter, req *http.Request) {
		_, _ = fmt.Fprintf(wr, "pong")
	}))

	httpRouter.Use(
		requestID(),
		requestLogger(),
	)

	zap.L().Info("starting http & grpc servers",
		zap.String("http_addr", httpAddr),
		zap.String("grpc_addr", grpcAddr),
	)
	return mux.Serve(ctx,
		mux.WithHTTPTarget(httpAddr, &http.Server{
			Handler:        httpRouter,
			ReadTimeout:    readTimeout,
			WriteTimeout:   writeTimeout,
			MaxHeaderBytes: maxHeaderBytes,
		}),
		mux.WithGRPCTarget(grpcAddr, grpcServer),
		mux.WithGracePeriod(gracePeriod),
	)
}

// moduleConfigLookup adapts the module service to masking.ModuleConfigLookup,
// exposing only a module's raw configs by URN.
type moduleConfigLookup struct {
	svc modulesv1.ModuleService
}

func (l moduleConfigLookup) ModuleConfigs(ctx context.Context, moduleURN string) (json.RawMessage, error) {
	mod, err := l.svc.GetModule(ctx, moduleURN)
	if err != nil {
		return nil, err
	}
	return mod.Configs, nil
}
