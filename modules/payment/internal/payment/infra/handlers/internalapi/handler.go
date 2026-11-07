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
	err := h.injectChaos(ctx, "ChargePayment", req.GetError())
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
	err := h.injectChaos(ctx, "RefundPayment", req.GetError())
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

// chaosSleep is how long a method listed in ChaosError.MethodSleep is delayed.
const chaosSleep = 15 * time.Second

// injectChaos delays or fails the method as requested, so callers can exercise retries and compensations.
func (h handler) injectChaos(ctx context.Context, method string, chaos *paymentinternalapi.ChaosError) error {
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
