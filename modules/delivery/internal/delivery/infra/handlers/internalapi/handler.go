package internalapi

import (
	"context"
	"log/slog"
	"slices"
	"time"
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
	err := h.injectChaos(ctx, "ScheduleDelivery", req.GetError())
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
	err := h.injectChaos(ctx, "CancelDelivery", req.GetError())
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

// chaosSleep is how long a method listed in ChaosError.MethodSleep is delayed.
const chaosSleep = 15 * time.Second

// injectChaos delays or fails the method as requested, so callers can exercise retries and compensations.
func (h handler) injectChaos(ctx context.Context, method string, chaos *deliveryinternalapi.ChaosError) error {
	if slices.Contains(chaos.GetMethodSleep(), method) {
		h.logger.WarnContext(ctx, "chaos sleep injected", "method", method, "duration", chaosSleep.String())
		select {
		case <-time.After(chaosSleep):
		case <-ctx.Done():
			return status.FromContextError(ctx.Err()).Err()
		}
	}
	if chaos.GetMethodError() == method {
		h.logger.WarnContext(ctx, "chaos error injected", "method", method)
		return status.Errorf(codes.Internal, "chaos: %s failed", method)
	}
	return nil
}
