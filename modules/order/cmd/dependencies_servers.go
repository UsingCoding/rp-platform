package main

import (
	"log/slog"

	gatewayapi "github.com/distributed-programming-2026/lib/gateway/interceptor"
	grpcserver "google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	"github.com/distributed-programming-2026/lib/runtime/grpc"
	"github.com/distributed-programming-2026/lib/runtime/http"

	internalapi "order/api/server/internalapi"
	"order/internal/order/app"
	"order/internal/order/config"
	gatewayhandler "order/internal/order/infra/handlers/gateway"
	internalhandler "order/internal/order/infra/handlers/internalapi"
	publichandler "order/internal/order/infra/handlers/public"
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
		internalapi.RegisterOrderServiceServer(server, internalhandler.NewHandler(svc, logger))
		gatewayapi.RegisterInterceptorServer(server, gatewayhandler.NewHandler())
		reflection.Register(server)
		return nil
	})
}
