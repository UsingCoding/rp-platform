package orderflow

import (
	"errors"
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

type OrderInput struct {
	OrderID string
	Amount  int64
	Address string
	// MethodError names the service method that must fail; used for chaos testing.
	MethodError string
}

type step struct {
	action       any
	compensation any
}

// CreateOrderWorkflow creates the order, charges it and schedules delivery.
// Each step is attempted up to 5 times. When a step fails, the workflow runs
// compensations for it and for all previous steps in reverse order.
func CreateOrderWorkflow(ctx workflow.Context, in OrderInput) error {
	ctx = workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		StartToCloseTimeout: 10 * time.Second,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval:    time.Second,
			BackoffCoefficient: 2,
			MaximumAttempts:    5,
		},
	})

	var a *Activities
	steps := []step{
		{action: a.CreateOrder, compensation: a.CancelOrder},
		{action: a.ChargePayment, compensation: a.RefundPayment},
		{action: a.ScheduleDelivery, compensation: a.CancelDelivery},
	}
	compensations := make([]any, 0, len(steps))
	for _, s := range steps {
		// Register the compensation before the step: a failed step may have been partly applied.
		// Compensations are idempotent, so compensating a step that changed nothing is safe.
		compensations = append(compensations, s.compensation)
		err := workflow.ExecuteActivity(ctx, s.action, in).Get(ctx, nil)
		if err != nil {
			return errors.Join(err, compensate(ctx, compensations, in))
		}
	}
	return nil
}

func compensate(ctx workflow.Context, compensations []any, in OrderInput) error {
	// Compensations must run even if the workflow is being cancelled.
	ctx, _ = workflow.NewDisconnectedContext(ctx)
	logger := workflow.GetLogger(ctx)
	var errs []error
	for i := len(compensations) - 1; i >= 0; i-- {
		err := workflow.ExecuteActivity(ctx, compensations[i], in).Get(ctx, nil)
		if err != nil {
			logger.Error("compensation failed", "error", err)
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}
