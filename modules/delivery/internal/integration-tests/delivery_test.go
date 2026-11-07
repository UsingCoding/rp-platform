package integration_tests

import (
	"context"
	"testing"
	"time"
	"uuid"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"

	internalapi "delivery/api/server/internalapi"
)

const testAddress = "Kazan"

func TestSchedule(t *testing.T) {
	connection, err := grpc.NewClient(testEnv.conf.GRPC.Address, grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	defer connection.Close()

	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()

	client := internalapi.NewDeliveryServiceClient(connection)
	orderID := uuid.New()
	request := &internalapi.ScheduleDeliveryRequest{
		OrderID: orderID.String(),
		Address: testAddress,
	}
	// The second call checks that retries are idempotent.
	for range 2 {
		_, err = client.ScheduleDelivery(ctx, request, grpc.WaitForReady(true))
		require.NoError(t, err)
	}

	var stored struct {
		Address string `db:"address"`
	}
	err = testEnv.connector.TransactionalClient().GetContext(t.Context(), "test.deliveries", &stored,
		"SELECT address FROM deliveries WHERE order_id = ?", orderID)
	require.NoError(t, err)
	require.Equal(t, testAddress, stored.Address)

	_, err = client.ScheduleDelivery(ctx, &internalapi.ScheduleDeliveryRequest{
		OrderID: "not-a-uuid",
		Address: testAddress,
	})
	require.Equal(t, codes.InvalidArgument, status.Code(err))
}

func TestScheduleDeliveryChaosAndCompensation(t *testing.T) {
	connection, err := grpc.NewClient(testEnv.conf.GRPC.Address, grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	defer connection.Close()

	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()

	client := internalapi.NewDeliveryServiceClient(connection)
	orderID := uuid.New()
	newRequest := func(methodError string) *internalapi.ScheduleDeliveryRequest {
		return &internalapi.ScheduleDeliveryRequest{
			OrderID: orderID.String(),
			Address: testAddress,
			Error:   &internalapi.ChaosError{MethodError: methodError},
		}
	}
	db := testEnv.connector.TransactionalClient()

	_, err = client.ScheduleDelivery(ctx, newRequest("ScheduleDelivery"), grpc.WaitForReady(true))
	require.Equal(t, codes.Internal, status.Code(err))
	var count int
	err = db.GetContext(t.Context(), "test.deliveries.count", &count, "SELECT COUNT(*) FROM deliveries WHERE order_id = ?", orderID)
	require.NoError(t, err)
	require.Zero(t, count)

	// A chaos error aimed at another method does not affect this one.
	_, err = client.ScheduleDelivery(ctx, newRequest("CancelDelivery"))
	require.NoError(t, err)

	_, err = client.CancelDelivery(ctx, &internalapi.CancelDeliveryRequest{
		OrderID: orderID.String(),
		Error:   &internalapi.ChaosError{MethodError: "CancelDelivery"},
	})
	require.Equal(t, codes.Internal, status.Code(err))

	for range 2 {
		_, err = client.CancelDelivery(ctx, &internalapi.CancelDeliveryRequest{OrderID: orderID.String()})
		require.NoError(t, err)
	}
	var orderStatus string
	err = db.GetContext(t.Context(), "test.deliveries.status", &orderStatus, "SELECT status FROM deliveries WHERE order_id = ?", orderID)
	require.NoError(t, err)
	require.Equal(t, "cancelled", orderStatus)
}

func TestScheduleDeliveryChaosSleep(t *testing.T) {
	connection, err := grpc.NewClient(testEnv.conf.GRPC.Address, grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	defer connection.Close()

	client := internalapi.NewDeliveryServiceClient(connection)
	orderID := uuid.New()
	newRequest := func(methodSleep ...string) *internalapi.ScheduleDeliveryRequest {
		return &internalapi.ScheduleDeliveryRequest{
			OrderID: orderID.String(),
			Address: testAddress,
			Error:   &internalapi.ChaosError{MethodSleep: methodSleep},
		}
	}

	// The sleep outlasts the caller's deadline, so the method gives up without writing.
	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()
	_, err = client.ScheduleDelivery(ctx, newRequest("CancelDelivery", "ScheduleDelivery"), grpc.WaitForReady(true))
	require.Equal(t, codes.DeadlineExceeded, status.Code(err))
	var count int
	err = testEnv.connector.TransactionalClient().GetContext(t.Context(), "test.deliveries.count", &count,
		"SELECT COUNT(*) FROM deliveries WHERE order_id = ?", orderID)
	require.NoError(t, err)
	require.Zero(t, count)

	// A sleep requested for another method does not delay this one.
	ctx, cancel = context.WithTimeout(t.Context(), time.Second)
	defer cancel()
	_, err = client.ScheduleDelivery(ctx, newRequest("CancelDelivery"))
	require.NoError(t, err)
}
