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
		Address: "Kazan",
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
	require.Equal(t, "Kazan", stored.Address)

	_, err = client.CreateOrder(ctx, &internalapi.CreateOrderRequest{
		OrderID: "not-a-uuid",
		Amount:  100,
		Address: "Kazan",
	})
	require.Equal(t, codes.InvalidArgument, status.Code(err))
}
