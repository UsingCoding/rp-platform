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

func (h handler) ScheduleDelivery(ctx context.Context, req *deliveryinternalapi.ScheduleDeliveryRequest) (*deliveryinternalapi.ScheduleDeliveryResponse, error) {
	err := h.chaosError(ctx, "ScheduleDelivery", req.GetError())
	if err != nil {
		return nil, err
	}

	orderID, err := uuid.Parse(req.OrderID)
	if err != nil || req.Address == "" {
		return nil, status.Error(codes.InvalidArgument, "invalid delivery request")
	}

	err = h.svc.ScheduleDelivery(ctx, domain.Delivery{
		OrderID: orderID,
		Address: req.Address,
	})
	if err != nil {
		h.logger.ErrorContext(ctx, "delivery failed", logging.Error(err))
		return nil, status.Error(codes.Internal, "delivery failed")
	}
	return &deliveryinternalapi.ScheduleDeliveryResponse{}, nil
}

func (h handler) CancelDelivery(ctx context.Context, req *deliveryinternalapi.CancelDeliveryRequest) (*deliveryinternalapi.CancelDeliveryResponse, error) {
	err := h.chaosError(ctx, "CancelDelivery", req.GetError())
	if err != nil {
		return nil, err
	}

	orderID, err := uuid.Parse(req.OrderID)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid delivery cancel request")
	}

	err = h.svc.CancelDelivery(ctx, orderID)
	if err != nil {
		h.logger.ErrorContext(ctx, "delivery cancel failed", logging.Error(err))
		return nil, status.Error(codes.Internal, "delivery cancel failed")
	}
	return &deliveryinternalapi.CancelDeliveryResponse{}, nil
}

// chaosError fails the method named in the request, so callers can exercise retries and compensations.
func (h handler) chaosError(ctx context.Context, method string, chaos *deliveryinternalapi.ChaosError) error {
	if chaos.GetMethodError() != method {
		return nil
	}
	h.logger.WarnContext(ctx, "chaos error injected", "method", method)
	return status.Errorf(codes.Internal, "chaos: %s failed", method)
}
