package internalapi

import (
	"context"
	"log/slog"

	"github.com/distributed-programming-2026/go-sdk/pkg/logging"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	ordercreatorinternalapi "ordercreator/api/server/internalapi"
	"ordercreator/internal/ordercreator/app"
)

func NewHandler(svc *app.Service, logger *slog.Logger) ordercreatorinternalapi.OrdercreatorServiceServer {
	return &handler{
		svc:    svc,
		logger: logger,
	}
}

type handler struct {
	svc    *app.Service
	logger *slog.Logger
	ordercreatorinternalapi.UnimplementedOrdercreatorServiceServer
}

func (h handler) Echo(ctx context.Context, req *ordercreatorinternalapi.EchoRequest) (*ordercreatorinternalapi.EchoResponse, error) {
	id, err := h.svc.Echo(ctx, req.Body)
	if err != nil {
		h.logger.ErrorContext(ctx, "echo failed", logging.Error(err))
		return nil, status.Error(codes.Internal, "echo failed")
	}

	return &ordercreatorinternalapi.EchoResponse{
		OperationID: id.String(),
	}, nil
}
