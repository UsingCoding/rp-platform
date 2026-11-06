package orderflow

import (
	"context"

	enumspb "go.temporal.io/api/enums/v1"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"

	"ordercreator/internal/ordercreator/domain"
)

func NewStarter(c client.Client, taskQueue string) *Starter {
	return &Starter{
		client:    c,
		taskQueue: taskQueue,
	}
}

type Starter struct {
	client    client.Client
	taskQueue string
}

func (s *Starter) StartCreateOrder(ctx context.Context, order domain.Order) (string, error) {
	run, err := s.client.ExecuteWorkflow(ctx, client.StartWorkflowOptions{
		ID:                    "order-" + order.ID.String(),
		TaskQueue:             s.taskQueue,
		WorkflowIDReusePolicy: enumspb.WORKFLOW_ID_REUSE_POLICY_REJECT_DUPLICATE,
	}, CreateOrderWorkflow, OrderInput{
		OrderID: order.ID.String(),
		Amount:  order.Amount,
		Address: order.Address,
	})
	if err != nil {
		return "", err
	}
	return run.GetID(), nil
}

func Register(registry worker.Registry, activities *Activities) {
	registry.RegisterWorkflow(CreateOrderWorkflow)
	registry.RegisterActivity(activities)
}
