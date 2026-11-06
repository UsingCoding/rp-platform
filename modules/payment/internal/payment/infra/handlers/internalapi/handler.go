package internalapi

import (
	"context"
	"log/slog"
	"uuid"

	"github.com/distributed-programming-2026/go-sdk/pkg/logging"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	paymentinternalapi "payment/api/server/internalapi"
	"payment/internal/payment/app"
	"payment/internal/payment/domain"
)

func NewHandler(svc *app.Service, logger *slog.Logger) paymentinternalapi.PaymentServiceServer {
	return &handler{
		svc:    svc,
		logger: logger,
	}
}

type handler struct {
	svc    *app.Service
	logger *slog.Logger
	paymentinternalapi.UnimplementedPaymentServiceServer
}

func (h handler) Echo(ctx context.Context, req *paymentinternalapi.EchoRequest) (*paymentinternalapi.EchoResponse, error) {
	id, err := h.svc.Echo(ctx, req.Body)
	if err != nil {
		h.logger.ErrorContext(ctx, "echo failed", logging.Error(err))
		return nil, status.Error(codes.Internal, "echo failed")
	}

	return &paymentinternalapi.EchoResponse{
		OperationID: id.String(),
	}, nil
}

func (h handler) Charge(ctx context.Context, req *paymentinternalapi.ChargeRequest) (*paymentinternalapi.ChargeResponse, error) {
	orderID, err := uuid.Parse(req.OrderID)
	if err != nil || req.Amount <= 0 {
		return nil, status.Error(codes.InvalidArgument, "invalid payment request")
	}

	err = h.svc.Charge(ctx, domain.Payment{
		OrderID: orderID,
		Amount:  req.Amount,
	})
	if err != nil {
		h.logger.ErrorContext(ctx, "payment failed", logging.Error(err))
		return nil, status.Error(codes.Internal, "payment failed")
	}
	return &paymentinternalapi.ChargeResponse{}, nil
}
