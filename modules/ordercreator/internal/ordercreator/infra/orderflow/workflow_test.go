package orderflow

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/testsuite"
)

var input = OrderInput{
	OrderID: "0b6f0d3c-4d4e-4a4f-9d5a-6c1f2f3e4a5b",
	Amount:  100,
	Address: "Kazan",
}

func TestCreateOrderWorkflowRunsActivitiesInOrder(t *testing.T) {
	var suite testsuite.WorkflowTestSuite
	env := suite.NewTestWorkflowEnvironment()
	var a *Activities
	env.RegisterActivity(a)

	var calls []string
	record := func(name string) func(mock.Arguments) {
		return func(mock.Arguments) {
			calls = append(calls, name)
		}
	}
	env.OnActivity(a.CreateOrder, mock.Anything, input).Return(nil).Run(record("CreateOrder"))
	env.OnActivity(a.ChargePayment, mock.Anything, input).Return(nil).Run(record("ChargePayment"))
	env.OnActivity(a.ScheduleDelivery, mock.Anything, input).Return(nil).Run(record("ScheduleDelivery"))

	env.ExecuteWorkflow(CreateOrderWorkflow, input)

	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())
	require.Equal(t, []string{"CreateOrder", "ChargePayment", "ScheduleDelivery"}, calls)
}

func TestCreateOrderWorkflowStopsOnFailure(t *testing.T) {
	var suite testsuite.WorkflowTestSuite
	env := suite.NewTestWorkflowEnvironment()
	var a *Activities
	env.RegisterActivity(a)

	env.OnActivity(a.CreateOrder, mock.Anything, input).Return(nil)
	env.OnActivity(a.ChargePayment, mock.Anything, input).
		Return(temporal.NewNonRetryableApplicationError("declined", "InvalidArgument", errors.New("declined")))

	env.ExecuteWorkflow(CreateOrderWorkflow, input)

	require.True(t, env.IsWorkflowCompleted())
	require.Error(t, env.GetWorkflowError())
	env.AssertNotCalled(t, "ScheduleDelivery", mock.Anything, mock.Anything)
}

func TestCreateOrderWorkflowRetriesTransientFailure(t *testing.T) {
	var suite testsuite.WorkflowTestSuite
	env := suite.NewTestWorkflowEnvironment()
	var a *Activities
	env.RegisterActivity(a)

	env.OnActivity(a.CreateOrder, mock.Anything, input).Return(errors.New("unavailable")).Once()
	env.OnActivity(a.CreateOrder, mock.Anything, input).Return(nil).Once()
	env.OnActivity(a.ChargePayment, mock.Anything, input).Return(nil)
	env.OnActivity(a.ScheduleDelivery, mock.Anything, input).Return(nil)

	env.ExecuteWorkflow(CreateOrderWorkflow, input)

	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())
	env.AssertExpectations(t)
}
