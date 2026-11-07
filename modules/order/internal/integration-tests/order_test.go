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

	internalapi "order/api/server/internalapi"
)

const testAddress = "Kazan"

func TestCreateOrder(t *testing.T) {
	connection, err := grpc.NewClient(testEnv.conf.GRPC.Address, grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	defer connection.Close()

	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()

	client := internalapi.NewOrderServiceClient(connection)
	orderID := uuid.New()
	request := &internalapi.CreateOrderRequest{
		OrderID: orderID.String(),
		Amount:  100,
		Address: testAddress,
	}
	// The second call checks that retries are idempotent.
	for range 2 {
		_, err = client.CreateOrder(ctx, request, grpc.WaitForReady(true))
		require.NoError(t, err)
	}

	var stored struct {
		Amount  int64  `db:"amount"`
		Address string `db:"address"`
	}
	err = testEnv.connector.TransactionalClient().GetContext(t.Context(), "test.orders", &stored,
		"SELECT amount, address FROM orders WHERE order_id = ?", orderID)
	require.NoError(t, err)
	require.Equal(t, int64(100), stored.Amount)
	require.Equal(t, testAddress, stored.Address)

	_, err = client.CreateOrder(ctx, &internalapi.CreateOrderRequest{
		OrderID: "not-a-uuid",
		Amount:  100,
		Address: testAddress,
	})
	require.Equal(t, codes.InvalidArgument, status.Code(err))
}

func TestCreateOrderChaosAndCompensation(t *testing.T) {
	connection, err := grpc.NewClient(testEnv.conf.GRPC.Address, grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	defer connection.Close()

	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()

	client := internalapi.NewOrderServiceClient(connection)
	orderID := uuid.New()
	newRequest := func(methodError string) *internalapi.CreateOrderRequest {
		return &internalapi.CreateOrderRequest{
			OrderID: orderID.String(),
			Amount:  100,
			Address: testAddress,
			Error:   &internalapi.ChaosError{MethodError: methodError},
		}
	}
	db := testEnv.connector.TransactionalClient()

	_, err = client.CreateOrder(ctx, newRequest("CreateOrder"), grpc.WaitForReady(true))
	require.Equal(t, codes.Internal, status.Code(err))
	var count int
	err = db.GetContext(t.Context(), "test.orders.count", &count, "SELECT COUNT(*) FROM orders WHERE order_id = ?", orderID)
	require.NoError(t, err)
	require.Zero(t, count)

	// A chaos error aimed at another method does not affect this one.
	_, err = client.CreateOrder(ctx, newRequest("CancelOrder"))
	require.NoError(t, err)

	_, err = client.CancelOrder(ctx, &internalapi.CancelOrderRequest{
		OrderID: orderID.String(),
		Error:   &internalapi.ChaosError{MethodError: "CancelOrder"},
	})
	require.Equal(t, codes.Internal, status.Code(err))

	for range 2 {
		_, err = client.CancelOrder(ctx, &internalapi.CancelOrderRequest{OrderID: orderID.String()})
		require.NoError(t, err)
	}
	var orderStatus string
	err = db.GetContext(t.Context(), "test.orders.status", &orderStatus, "SELECT status FROM orders WHERE order_id = ?", orderID)
	require.NoError(t, err)
	require.Equal(t, "cancelled", orderStatus)
}

func TestCreateOrderChaosSleep(t *testing.T) {
	connection, err := grpc.NewClient(testEnv.conf.GRPC.Address, grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	defer connection.Close()

	client := internalapi.NewOrderServiceClient(connection)
	orderID := uuid.New()
	newRequest := func(methodSleep ...string) *internalapi.CreateOrderRequest {
		return &internalapi.CreateOrderRequest{
			OrderID: orderID.String(),
			Amount:  100,
			Address: testAddress,
			Error:   &internalapi.ChaosError{MethodSleep: methodSleep},
		}
	}

	// The sleep outlasts the caller's deadline, so the method gives up without writing.
	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()
	_, err = client.CreateOrder(ctx, newRequest("CancelOrder", "CreateOrder"), grpc.WaitForReady(true))
	require.Equal(t, codes.DeadlineExceeded, status.Code(err))
	var count int
	err = testEnv.connector.TransactionalClient().GetContext(t.Context(), "test.orders.count", &count,
		"SELECT COUNT(*) FROM orders WHERE order_id = ?", orderID)
	require.NoError(t, err)
	require.Zero(t, count)

	// A sleep requested for another method does not delay this one.
	ctx, cancel = context.WithTimeout(t.Context(), time.Second)
	defer cancel()
	_, err = client.CreateOrder(ctx, newRequest("CancelOrder"))
	require.NoError(t, err)
}
