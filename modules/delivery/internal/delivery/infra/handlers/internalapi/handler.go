package internalapi

import (
	"context"
	"log/slog"
	"uuid"

	"github.com/distributed-programming-2026/go-sdk/pkg/logging"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	deliveryinternalapi "delivery/api/server/internalapi"
	"delivery/internal/delivery/app"
	"delivery/internal/delivery/domain"
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

func (h handler) Schedule(ctx context.Context, req *deliveryinternalapi.ScheduleRequest) (*deliveryinternalapi.ScheduleResponse, error) {
	orderID, err := uuid.Parse(req.OrderID)
	if err != nil || req.Address == "" {
		return nil, status.Error(codes.InvalidArgument, "invalid delivery request")
	}

	err = h.svc.Schedule(ctx, domain.Delivery{
		OrderID: orderID,
		Address: req.Address,
	})
	if err != nil {
		h.logger.ErrorContext(ctx, "delivery failed", logging.Error(err))
		return nil, status.Error(codes.Internal, "delivery failed")
	}
	return &deliveryinternalapi.ScheduleResponse{}, nil
}
