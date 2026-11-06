package internalapi

import (
	"context"
	"log/slog"

	"github.com/distributed-programming-2026/go-sdk/pkg/logging"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	deliveryinternalapi "delivery/api/server/internalapi"
	"delivery/internal/delivery/app"
)

func NewHandler(svc *app.Service, logger *slog.Logger) deliveryinternalapi.DeliveryServiceServer {
	return &handler{
		svc:    svc,
		logger: logger,
	}
}

type handler struct {
	svc    *app.Service
	logger *slog.Logger
	deliveryinternalapi.UnimplementedDeliveryServiceServer
}

func (h handler) Echo(ctx context.Context, req *deliveryinternalapi.EchoRequest) (*deliveryinternalapi.EchoResponse, error) {
	id, err := h.svc.Echo(ctx, req.Body)
	if err != nil {
		h.logger.ErrorContext(ctx, "echo failed", logging.Error(err))
		return nil, status.Error(codes.Internal, "echo failed")
	}

	return &deliveryinternalapi.EchoResponse{
		OperationID: id.String(),
	}, nil
}
