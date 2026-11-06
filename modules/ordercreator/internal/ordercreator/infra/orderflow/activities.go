package orderflow

import (
	"context"

	"go.temporal.io/sdk/temporal"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	deliveryinternalapi "delivery/api/server/internalapi"
	orderinternalapi "order/api/server/internalapi"
	paymentinternalapi "payment/api/server/internalapi"
)

func NewActivities(
	orders orderinternalapi.OrderServiceClient,
	payments paymentinternalapi.PaymentServiceClient,
	deliveries deliveryinternalapi.DeliveryServiceClient,
) *Activities {
	return &Activities{
		orders:     orders,
		payments:   payments,
		deliveries: deliveries,
	}
}

type Activities struct {
	orders     orderinternalapi.OrderServiceClient
	payments   paymentinternalapi.PaymentServiceClient
	deliveries deliveryinternalapi.DeliveryServiceClient
}

func (a *Activities) CreateOrder(ctx context.Context, in OrderInput) error {
	_, err := a.orders.CreateOrder(ctx, &orderinternalapi.CreateOrderRequest{
		OrderID: in.OrderID,
		Amount:  in.Amount,
		Address: in.Address,
		Error:   &orderinternalapi.ChaosError{MethodError: in.MethodError},
	})
	return activityError(err)
}

func (a *Activities) CancelOrder(ctx context.Context, in OrderInput) error {
	_, err := a.orders.CancelOrder(ctx, &orderinternalapi.CancelOrderRequest{
		OrderID: in.OrderID,
		Error:   &orderinternalapi.ChaosError{MethodError: in.MethodError},
	})
	return activityError(err)
}

func (a *Activities) ChargePayment(ctx context.Context, in OrderInput) error {
	_, err := a.payments.ChargePayment(ctx, &paymentinternalapi.ChargePaymentRequest{
		OrderID: in.OrderID,
		Amount:  in.Amount,
		Error:   &paymentinternalapi.ChaosError{MethodError: in.MethodError},
	})
	return activityError(err)
}

func (a *Activities) RefundPayment(ctx context.Context, in OrderInput) error {
	_, err := a.payments.RefundPayment(ctx, &paymentinternalapi.RefundPaymentRequest{
		OrderID: in.OrderID,
		Error:   &paymentinternalapi.ChaosError{MethodError: in.MethodError},
	})
	return activityError(err)
}

func (a *Activities) ScheduleDelivery(ctx context.Context, in OrderInput) error {
	_, err := a.deliveries.ScheduleDelivery(ctx, &deliveryinternalapi.ScheduleDeliveryRequest{
		OrderID: in.OrderID,
		Address: in.Address,
		Error:   &deliveryinternalapi.ChaosError{MethodError: in.MethodError},
	})
	return activityError(err)
}

func (a *Activities) CancelDelivery(ctx context.Context, in OrderInput) error {
	_, err := a.deliveries.CancelDelivery(ctx, &deliveryinternalapi.CancelDeliveryRequest{
		OrderID: in.OrderID,
		Error:   &deliveryinternalapi.ChaosError{MethodError: in.MethodError},
	})
	return activityError(err)
}

// activityError stops retries for requests the downstream service rejects as invalid.
func activityError(err error) error {
	if err == nil {
		return nil
	}
	if status.Code(err) == codes.InvalidArgument {
		return temporal.NewNonRetryableApplicationError(err.Error(), codes.InvalidArgument.String(), err)
	}
	return err
}
