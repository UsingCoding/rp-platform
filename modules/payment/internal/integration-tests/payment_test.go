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

	internalapi "payment/api/server/internalapi"
)

func TestCharge(t *testing.T) {
	connection, err := grpc.NewClient(testEnv.conf.GRPC.Address, grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	defer connection.Close()

	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()

	client := internalapi.NewPaymentServiceClient(connection)
	orderID := uuid.New()
	request := &internalapi.ChargePaymentRequest{
		OrderID: orderID.String(),
		Amount:  100,
	}
	// The second call checks that retries are idempotent.
	for range 2 {
		_, err = client.ChargePayment(ctx, request, grpc.WaitForReady(true))
		require.NoError(t, err)
	}

	var stored struct {
		Amount int64 `db:"amount"`
	}
	err = testEnv.connector.TransactionalClient().GetContext(t.Context(), "test.payments", &stored,
		"SELECT amount FROM payments WHERE order_id = ?", orderID)
	require.NoError(t, err)
	require.Equal(t, int64(100), stored.Amount)

	_, err = client.ChargePayment(ctx, &internalapi.ChargePaymentRequest{
		OrderID: "not-a-uuid",
		Amount:  100,
	})
	require.Equal(t, codes.InvalidArgument, status.Code(err))
}

func TestChargePaymentChaosAndCompensation(t *testing.T) {
	connection, err := grpc.NewClient(testEnv.conf.GRPC.Address, grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	defer connection.Close()

	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()

	client := internalapi.NewPaymentServiceClient(connection)
	orderID := uuid.New()
	newRequest := func(methodError string) *internalapi.ChargePaymentRequest {
		return &internalapi.ChargePaymentRequest{
			OrderID: orderID.String(),
			Amount:  100,
			Error:   &internalapi.ChaosError{MethodError: methodError},
		}
	}
	db := testEnv.connector.TransactionalClient()

	_, err = client.ChargePayment(ctx, newRequest("ChargePayment"), grpc.WaitForReady(true))
	require.Equal(t, codes.Internal, status.Code(err))
	var count int
	err = db.GetContext(t.Context(), "test.payments.count", &count, "SELECT COUNT(*) FROM payments WHERE order_id = ?", orderID)
	require.NoError(t, err)
	require.Zero(t, count)

	// A chaos error aimed at another method does not affect this one.
	_, err = client.ChargePayment(ctx, newRequest("RefundPayment"))
	require.NoError(t, err)

	_, err = client.RefundPayment(ctx, &internalapi.RefundPaymentRequest{
		OrderID: orderID.String(),
		Error:   &internalapi.ChaosError{MethodError: "RefundPayment"},
	})
	require.Equal(t, codes.Internal, status.Code(err))

	for range 2 {
		_, err = client.RefundPayment(ctx, &internalapi.RefundPaymentRequest{OrderID: orderID.String()})
		require.NoError(t, err)
	}
	var orderStatus string
	err = db.GetContext(t.Context(), "test.payments.status", &orderStatus, "SELECT status FROM payments WHERE order_id = ?", orderID)
	require.NoError(t, err)
	require.Equal(t, "cancelled", orderStatus)
}

func TestChargePaymentChaosSleep(t *testing.T) {
	connection, err := grpc.NewClient(testEnv.conf.GRPC.Address, grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	defer connection.Close()

	client := internalapi.NewPaymentServiceClient(connection)
	orderID := uuid.New()
	newRequest := func(methodSleep ...string) *internalapi.ChargePaymentRequest {
		return &internalapi.ChargePaymentRequest{
			OrderID: orderID.String(),
			Amount:  100,
			Error:   &internalapi.ChaosError{MethodSleep: methodSleep},
		}
	}

	// The sleep outlasts the caller's deadline, so the method gives up without writing.
	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()
	_, err = client.ChargePayment(ctx, newRequest("RefundPayment", "ChargePayment"), grpc.WaitForReady(true))
	require.Equal(t, codes.DeadlineExceeded, status.Code(err))
	var count int
	err = testEnv.connector.TransactionalClient().GetContext(t.Context(), "test.payments.count", &count,
		"SELECT COUNT(*) FROM payments WHERE order_id = ?", orderID)
	require.NoError(t, err)
	require.Zero(t, count)

	// A sleep requested for another method does not delay this one.
	ctx, cancel = context.WithTimeout(t.Context(), time.Second)
	defer cancel()
	_, err = client.ChargePayment(ctx, newRequest("RefundPayment"))
	require.NoError(t, err)
}
