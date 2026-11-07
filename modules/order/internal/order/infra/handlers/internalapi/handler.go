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

	orderinternalapi "order/api/server/internalapi"
	"order/internal/order/app"
	"order/internal/order/domain"
)

func NewHandler(svc *app.Service, logger *slog.Logger) orderinternalapi.OrderServiceServer {
	return &handler{
		svc:    svc,
		logger: logger,
	}
}

type handler struct {
	svc    *app.Service
	logger *slog.Logger
	orderinternalapi.UnimplementedOrderServiceServer
}

func (h handler) Echo(ctx context.Context, req *orderinternalapi.EchoRequest) (*orderinternalapi.EchoResponse, error) {
	id, err := h.svc.Echo(ctx, req.Body)
	if err != nil {
		h.logger.ErrorContext(ctx, "echo failed", logging.Error(err))
		return nil, status.Error(codes.Internal, "echo failed")
	}

	return &orderinternalapi.EchoResponse{
		OperationID: id.String(),
	}, nil
}

func (h handler) CreateOrder(ctx context.Context, req *orderinternalapi.CreateOrderRequest) (*orderinternalapi.CreateOrderResponse, error) {
	err := h.injectChaos(ctx, "CreateOrder", req.GetError())
	if err != nil {
		return nil, err
	}

	orderID, err := uuid.Parse(req.OrderID)
	if err != nil || req.Amount <= 0 || req.Address == "" {
		return nil, status.Error(codes.InvalidArgument, "invalid order request")
	}

	err = h.svc.CreateOrder(ctx, domain.Order{
		OrderID: orderID,
		Amount:  req.Amount,
		Address: req.Address,
	})
	if err != nil {
		h.logger.ErrorContext(ctx, "order failed", logging.Error(err))
		return nil, status.Error(codes.Internal, "order failed")
	}
	return &orderinternalapi.CreateOrderResponse{}, nil
}

func (h handler) CancelOrder(ctx context.Context, req *orderinternalapi.CancelOrderRequest) (*orderinternalapi.CancelOrderResponse, error) {
	err := h.injectChaos(ctx, "CancelOrder", req.GetError())
	if err != nil {
		return nil, err
	}

	orderID, err := uuid.Parse(req.OrderID)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid order cancel request")
	}

	err = h.svc.CancelOrder(ctx, orderID)
	if err != nil {
		h.logger.ErrorContext(ctx, "order cancel failed", logging.Error(err))
		return nil, status.Error(codes.Internal, "order cancel failed")
	}
	return &orderinternalapi.CancelOrderResponse{}, nil
}

// chaosSleep is how long a method listed in ChaosError.MethodSleep is delayed.
const chaosSleep = 15 * time.Second

// injectChaos delays or fails the method as requested, so callers can exercise retries and compensations.
func (h handler) injectChaos(ctx context.Context, method string, chaos *orderinternalapi.ChaosError) error {
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
