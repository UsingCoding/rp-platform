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

func (h handler) ChargePayment(ctx context.Context, req *paymentinternalapi.ChargePaymentRequest) (*paymentinternalapi.ChargePaymentResponse, error) {
	err := h.chaosError(ctx, "ChargePayment", req.GetError())
	if err != nil {
		return nil, err
	}

	orderID, err := uuid.Parse(req.OrderID)
	if err != nil || req.Amount <= 0 {
		return nil, status.Error(codes.InvalidArgument, "invalid payment request")
	}

	err = h.svc.ChargePayment(ctx, domain.Payment{
		OrderID: orderID,
		Amount:  req.Amount,
	})
	if err != nil {
		h.logger.ErrorContext(ctx, "payment failed", logging.Error(err))
		return nil, status.Error(codes.Internal, "payment failed")
	}
	return &paymentinternalapi.ChargePaymentResponse{}, nil
}

func (h handler) RefundPayment(ctx context.Context, req *paymentinternalapi.RefundPaymentRequest) (*paymentinternalapi.RefundPaymentResponse, error) {
	err := h.chaosError(ctx, "RefundPayment", req.GetError())
	if err != nil {
		return nil, err
	}

	orderID, err := uuid.Parse(req.OrderID)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid payment cancel request")
	}

	err = h.svc.RefundPayment(ctx, orderID)
	if err != nil {
		h.logger.ErrorContext(ctx, "payment cancel failed", logging.Error(err))
		return nil, status.Error(codes.Internal, "payment cancel failed")
	}
	return &paymentinternalapi.RefundPaymentResponse{}, nil
}

// chaosError fails the method named in the request, so callers can exercise retries and compensations.
func (h handler) chaosError(ctx context.Context, method string, chaos *paymentinternalapi.ChaosError) error {
	if chaos.GetMethodError() != method {
		return nil
	}
	h.logger.WarnContext(ctx, "chaos error injected", "method", method)
	return status.Errorf(codes.Internal, "chaos: %s failed", method)
}
