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

type orderServer struct {
	orderinternalapi.UnimplementedOrderServiceServer
	requests []*orderinternalapi.CreateOrderRequest
	err      error
}

func (s *orderServer) CreateOrder(_ context.Context, req *orderinternalapi.CreateOrderRequest) (*orderinternalapi.CreateOrderResponse, error) {
	s.requests = append(s.requests, req)
	return &orderinternalapi.CreateOrderResponse{}, s.err
}

type paymentServer struct {
	paymentinternalapi.UnimplementedPaymentServiceServer
	requests []*paymentinternalapi.ChargeRequest
}

func (s *paymentServer) Charge(_ context.Context, req *paymentinternalapi.ChargeRequest) (*paymentinternalapi.ChargeResponse, error) {
	s.requests = append(s.requests, req)
	return &paymentinternalapi.ChargeResponse{}, nil
}

type deliveryServer struct {
	deliveryinternalapi.UnimplementedDeliveryServiceServer
	requests []*deliveryinternalapi.ScheduleRequest
}

func (s *deliveryServer) Schedule(_ context.Context, req *deliveryinternalapi.ScheduleRequest) (*deliveryinternalapi.ScheduleResponse, error) {
	s.requests = append(s.requests, req)
	return &deliveryinternalapi.ScheduleResponse{}, nil
}

type fakeServices struct {
	orders     *orderServer
	payments   *paymentServer
	deliveries *deliveryServer
	activities *Activities
}

func startFakeServices(t *testing.T) *fakeServices {
	t.Helper()
	listener := bufconn.Listen(1 << 20)
	server := grpc.NewServer()
	services := &fakeServices{
		orders:     &orderServer{},
		payments:   &paymentServer{},
		deliveries: &deliveryServer{},
	}
	orderinternalapi.RegisterOrderServiceServer(server, services.orders)
	paymentinternalapi.RegisterPaymentServiceServer(server, services.payments)
	deliveryinternalapi.RegisterDeliveryServiceServer(server, services.deliveries)
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

	require.NoError(t, services.activities.CreateOrder(t.Context(), input))
	require.NoError(t, services.activities.ChargePayment(t.Context(), input))
	require.NoError(t, services.activities.ScheduleDelivery(t.Context(), input))

	require.Len(t, services.orders.requests, 1)
	require.Equal(t, input.OrderID, services.orders.requests[0].OrderID)
	require.Equal(t, input.Amount, services.orders.requests[0].Amount)
	require.Equal(t, input.Address, services.orders.requests[0].Address)
	require.Len(t, services.payments.requests, 1)
	require.Equal(t, input.Amount, services.payments.requests[0].Amount)
	require.Len(t, services.deliveries.requests, 1)
	require.Equal(t, input.Address, services.deliveries.requests[0].Address)
}

func TestActivitiesErrors(t *testing.T) {
	services := startFakeServices(t)

	services.orders.err = status.Error(codes.InvalidArgument, "invalid order request")
	err := services.activities.CreateOrder(t.Context(), input)
	var applicationErr *temporal.ApplicationError
	require.ErrorAs(t, err, &applicationErr)
	require.True(t, applicationErr.NonRetryable())

	services.orders.err = status.Error(codes.Unavailable, "unavailable")
	err = services.activities.CreateOrder(t.Context(), input)
	require.Equal(t, codes.Unavailable, status.Code(err))
	require.NotErrorAs(t, err, &applicationErr)
}
