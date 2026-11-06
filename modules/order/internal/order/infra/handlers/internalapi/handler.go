package internalapi

import (
	"context"
	"log/slog"
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
