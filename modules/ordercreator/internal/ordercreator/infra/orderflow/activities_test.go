package orderflow

import (
	"context"
	"net"
	"testing"

	"github.com/stretchr/testify/require"
	"go.temporal.io/sdk/temporal"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"

	deliveryinternalapi "delivery/api/server/internalapi"
	orderinternalapi "order/api/server/internalapi"
	paymentinternalapi "payment/api/server/internalapi"
)

type request interface {
	GetOrderID() string
}

// fakeServices records requests sent to all three services and answers with err.
type fakeServices struct {
	requests   map[string]request
	err        error
	activities *Activities
}

func (s *fakeServices) record(method string, req request) error {
	s.requests[method] = req
	return s.err
}

type fakeOrders struct {
	orderinternalapi.UnimplementedOrderServiceServer
	*fakeServices
}

func (s fakeOrders) CreateOrder(_ context.Context, req *orderinternalapi.CreateOrderRequest) (*orderinternalapi.CreateOrderResponse, error) {
	return &orderinternalapi.CreateOrderResponse{}, s.record(createOrder, req)
}

func (s fakeOrders) CancelOrder(_ context.Context, req *orderinternalapi.CancelOrderRequest) (*orderinternalapi.CancelOrderResponse, error) {
	return &orderinternalapi.CancelOrderResponse{}, s.record(cancelOrder, req)
}

type fakePayments struct {
	paymentinternalapi.UnimplementedPaymentServiceServer
	*fakeServices
}

func (s fakePayments) ChargePayment(_ context.Context, req *paymentinternalapi.ChargePaymentRequest) (*paymentinternalapi.ChargePaymentResponse, error) {
	return &paymentinternalapi.ChargePaymentResponse{}, s.record(chargePayment, req)
}

func (s fakePayments) RefundPayment(_ context.Context, req *paymentinternalapi.RefundPaymentRequest) (*paymentinternalapi.RefundPaymentResponse, error) {
	return &paymentinternalapi.RefundPaymentResponse{}, s.record(refundPayment, req)
}

type fakeDeliveries struct {
	deliveryinternalapi.UnimplementedDeliveryServiceServer
	*fakeServices
}

func (s fakeDeliveries) ScheduleDelivery(_ context.Context, req *deliveryinternalapi.ScheduleDeliveryRequest) (*deliveryinternalapi.ScheduleDeliveryResponse, error) {
	return &deliveryinternalapi.ScheduleDeliveryResponse{}, s.record(scheduleDelivery, req)
}

func (s fakeDeliveries) CancelDelivery(_ context.Context, req *deliveryinternalapi.CancelDeliveryRequest) (*deliveryinternalapi.CancelDeliveryResponse, error) {
	return &deliveryinternalapi.CancelDeliveryResponse{}, s.record(cancelDelivery, req)
}

func startFakeServices(t *testing.T) *fakeServices {
	t.Helper()
	listener := bufconn.Listen(1 << 20)
	server := grpc.NewServer()
	services := &fakeServices{
		requests: map[string]request{},
	}
	orderinternalapi.RegisterOrderServiceServer(server, fakeOrders{fakeServices: services})
	paymentinternalapi.RegisterPaymentServiceServer(server, fakePayments{fakeServices: services})
	deliveryinternalapi.RegisterDeliveryServiceServer(server, fakeDeliveries{fakeServices: services})
	go func() {
		_ = server.Serve(listener)
	}()
	t.Cleanup(server.Stop)

	connection, err := grpc.NewClient("passthrough:///bufnet",
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
			return listener.DialContext(ctx)
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = connection.Close()
	})

	services.activities = NewActivities(
		orderinternalapi.NewOrderServiceClient(connection),
		paymentinternalapi.NewPaymentServiceClient(connection),
		deliveryinternalapi.NewDeliveryServiceClient(connection),
	)
	return services
}

func TestActivitiesCallServices(t *testing.T) {
	services := startFakeServices(t)
	a := services.activities
	in := input
	in.MethodError = scheduleDelivery
	in.MethodSleep = []string{chargePayment, cancelOrder}

	for _, activity := range []func(context.Context, OrderInput) error{
		a.CreateOrder, a.CancelOrder, a.ChargePayment, a.RefundPayment, a.ScheduleDelivery, a.CancelDelivery,
	} {
		require.NoError(t, activity(t.Context(), in))
	}

	order := services.requests[createOrder].(*orderinternalapi.CreateOrderRequest)
	require.Equal(t, in.Amount, order.Amount)
	require.Equal(t, in.Address, order.Address)
	require.Equal(t, in.Amount, services.requests[chargePayment].(*paymentinternalapi.ChargePaymentRequest).Amount)
	require.Equal(t, in.Address, services.requests[scheduleDelivery].(*deliveryinternalapi.ScheduleDeliveryRequest).Address)

	require.Len(t, services.requests, 6)
	for method, req := range services.requests {
		require.Equal(t, in.OrderID, req.GetOrderID(), method)
	}
	require.Equal(t, in.MethodError, order.GetError().GetMethodError())
	require.Equal(t, in.MethodSleep, order.GetError().GetMethodSleep())
	require.Equal(t, in.MethodSleep, services.requests[cancelDelivery].(*deliveryinternalapi.CancelDeliveryRequest).GetError().GetMethodSleep())
	require.Equal(t, in.MethodError, services.requests[cancelOrder].(*orderinternalapi.CancelOrderRequest).GetError().GetMethodError())
	require.Equal(t, in.MethodError, services.requests[chargePayment].(*paymentinternalapi.ChargePaymentRequest).GetError().GetMethodError())
	require.Equal(t, in.MethodError, services.requests[scheduleDelivery].(*deliveryinternalapi.ScheduleDeliveryRequest).GetError().GetMethodError())
	require.Equal(t, in.MethodError, services.requests[refundPayment].(*paymentinternalapi.RefundPaymentRequest).GetError().GetMethodError())
	require.Equal(t, in.MethodError, services.requests[cancelDelivery].(*deliveryinternalapi.CancelDeliveryRequest).GetError().GetMethodError())
}

func TestActivitiesErrors(t *testing.T) {
	services := startFakeServices(t)

	services.err = status.Error(codes.InvalidArgument, "invalid order request")
	err := services.activities.CreateOrder(t.Context(), input)
	var applicationErr *temporal.ApplicationError
	require.ErrorAs(t, err, &applicationErr)
	require.True(t, applicationErr.NonRetryable())

	services.err = status.Error(codes.Internal, "chaos: CreateOrder failed")
	err = services.activities.CreateOrder(t.Context(), input)
	require.Equal(t, codes.Internal, status.Code(err))
	require.NotErrorAs(t, err, &applicationErr)
}
