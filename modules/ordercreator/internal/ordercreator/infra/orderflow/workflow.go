package orderflow

import (
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

type OrderInput struct {
	OrderID string
	Amount  int64
	Address string
}

// CreateOrderWorkflow creates the order, charges it and schedules delivery.
// Each step is retried by Temporal; downstream services store rows idempotently.
func CreateOrderWorkflow(ctx workflow.Context, in OrderInput) error {
	ctx = workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		StartToCloseTimeout: 10 * time.Second,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval: time.Second,
			MaximumAttempts: 5,
		},
	})

	var a *Activities
	for _, activity := range []any{a.CreateOrder, a.ChargePayment, a.ScheduleDelivery} {
		err := workflow.ExecuteActivity(ctx, activity, in).Get(ctx, nil)
		if err != nil {
			return err
		}
	}
	return nil
}
