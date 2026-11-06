package main

import (
	"log/slog"

	gatewayapi "github.com/distributed-programming-2026/lib/gateway/interceptor"
	grpcserver "google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	"github.com/distributed-programming-2026/lib/runtime/grpc"
	"github.com/distributed-programming-2026/lib/runtime/http"

	internalapi "ordercreator/api/server/internalapi"
	"ordercreator/internal/ordercreator/app"
	"ordercreator/internal/ordercreator/config"
	gatewayhandler "ordercreator/internal/ordercreator/infra/handlers/gateway"
	internalhandler "ordercreator/internal/ordercreator/infra/handlers/internalapi"
	publichandler "ordercreator/internal/ordercreator/infra/handlers/public"
)

func newHTTPServer(logger *slog.Logger, conf config.Env, svc *app.Service) (*http.Server, error) {
	register, err := publichandler.Register(svc, logger)
	if err != nil {
		return nil, err
	}

	return http.NewServer(http.Config{
		Address:      conf.HTTP.Address,
		GraceTimeout: conf.App.GraceTimeout,
	}, register)
}

func newGRPCServer(logger *slog.Logger, conf config.Env, svc *app.Service) (*grpc.Server, error) {
	return grpc.NewServer(grpc.Config{
		Address:      conf.GRPC.Address,
		GraceTimeout: conf.App.GraceTimeout,
	}, func(server *grpcserver.Server) error {
		internalapi.RegisterOrdercreatorServiceServer(server, internalhandler.NewHandler(svc, logger))
		gatewayapi.RegisterInterceptorServer(server, gatewayhandler.NewHandler())
		reflection.Register(server)
		return nil
	})
}
