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
	request := &internalapi.ChargeRequest{
		OrderID: orderID.String(),
		Amount:  100,
	}
	// The second call checks that retries are idempotent.
	for range 2 {
		_, err = client.Charge(ctx, request, grpc.WaitForReady(true))
		require.NoError(t, err)
	}

	var stored struct {
		Amount int64 `db:"amount"`
	}
	err = testEnv.connector.TransactionalClient().GetContext(t.Context(), "test.payments", &stored,
		"SELECT amount FROM payments WHERE order_id = ?", orderID)
	require.NoError(t, err)
	require.Equal(t, int64(100), stored.Amount)

	_, err = client.Charge(ctx, &internalapi.ChargeRequest{
		OrderID: "not-a-uuid",
		Amount:  100,
	})
	require.Equal(t, codes.InvalidArgument, status.Code(err))
}
