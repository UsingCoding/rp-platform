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

func TestSchedule(t *testing.T) {
	connection, err := grpc.NewClient(testEnv.conf.GRPC.Address, grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	defer connection.Close()

	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()

	client := internalapi.NewDeliveryServiceClient(connection)
	orderID := uuid.New()
	request := &internalapi.ScheduleRequest{
		OrderID: orderID.String(),
		Address: "Kazan",
	}
	// The second call checks that retries are idempotent.
	for range 2 {
		_, err = client.Schedule(ctx, request, grpc.WaitForReady(true))
		require.NoError(t, err)
	}

	var stored struct {
		Address string `db:"address"`
	}
	err = testEnv.connector.TransactionalClient().GetContext(t.Context(), "test.deliveries", &stored,
		"SELECT address FROM deliveries WHERE order_id = ?", orderID)
	require.NoError(t, err)
	require.Equal(t, "Kazan", stored.Address)

	_, err = client.Schedule(ctx, &internalapi.ScheduleRequest{
		OrderID: "not-a-uuid",
		Address: "Kazan",
	})
	require.Equal(t, codes.InvalidArgument, status.Code(err))
}
