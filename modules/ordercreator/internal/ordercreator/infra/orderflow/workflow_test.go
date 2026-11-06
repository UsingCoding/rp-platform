package orderflow

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/testsuite"
)

// Activity names, which are also the names of the service methods they call.
const (
	createOrder      = "CreateOrder"
	cancelOrder      = "CancelOrder"
	chargePayment    = "ChargePayment"
	refundPayment    = "RefundPayment"
	scheduleDelivery = "ScheduleDelivery"
	cancelDelivery   = "CancelDelivery"
)

var input = OrderInput{
	OrderID: "0b6f0d3c-4d4e-4a4f-9d5a-6c1f2f3e4a5b",
	Amount:  100,
	Address: "Kazan",
}

type workflowTest struct {
	env   *testsuite.TestWorkflowEnvironment
	calls []string
}

// newWorkflowTest mocks every activity to succeed unless failures override it.
func newWorkflowTest(t *testing.T, failures map[string]error) *workflowTest {
	t.Helper()
	var suite testsuite.WorkflowTestSuite
	test := &workflowTest{env: suite.NewTestWorkflowEnvironment()}
	var a *Activities
	test.env.RegisterActivity(a)

	activities := map[string]any{
		createOrder:      a.CreateOrder,
		cancelOrder:      a.CancelOrder,
		chargePayment:    a.ChargePayment,
		refundPayment:    a.RefundPayment,
		scheduleDelivery: a.ScheduleDelivery,
		cancelDelivery:   a.CancelDelivery,
	}
	for name, activity := range activities {
		test.env.OnActivity(activity, mock.Anything, input).Return(failures[name]).Run(func(mock.Arguments) {
			test.calls = append(test.calls, name)
		})
	}
	return test
}

func (w *workflowTest) run(t *testing.T) error {
	t.Helper()
	w.env.ExecuteWorkflow(CreateOrderWorkflow, input)
	require.True(t, w.env.IsWorkflowCompleted())
	return w.env.GetWorkflowError()
}

func TestCreateOrderWorkflowSucceeds(t *testing.T) {
	test := newWorkflowTest(t, nil)

	require.NoError(t, test.run(t))
	require.Equal(t, []string{createOrder, chargePayment, scheduleDelivery}, test.calls)
}

func TestCreateOrderWorkflowCompensatesAfterRetries(t *testing.T) {
	test := newWorkflowTest(t, map[string]error{
		scheduleDelivery: errors.New("chaos: ScheduleDelivery failed"),
	})

	require.ErrorContains(t, test.run(t), "chaos: ScheduleDelivery failed")
	require.Equal(t, []string{
		createOrder, chargePayment,
		scheduleDelivery, scheduleDelivery, scheduleDelivery, scheduleDelivery, scheduleDelivery,
		cancelDelivery, refundPayment, cancelOrder,
	}, test.calls)
}

func TestCreateOrderWorkflowCompensatesOnlyStartedSteps(t *testing.T) {
	test := newWorkflowTest(t, map[string]error{
		createOrder: temporal.NewNonRetryableApplicationError("invalid", "InvalidArgument", nil),
	})

	require.Error(t, test.run(t))
	require.Equal(t, []string{createOrder, cancelOrder}, test.calls)
}

func TestCreateOrderWorkflowContinuesAfterFailedCompensation(t *testing.T) {
	test := newWorkflowTest(t, map[string]error{
		scheduleDelivery: temporal.NewNonRetryableApplicationError("schedule failed", "InvalidArgument", nil),
		refundPayment:    temporal.NewNonRetryableApplicationError("refund failed", "InvalidArgument", nil),
	})

	err := test.run(t)
	require.ErrorContains(t, err, "schedule failed")
	require.ErrorContains(t, err, "refund failed")
	require.Equal(t, []string{
		createOrder, chargePayment, scheduleDelivery,
		cancelDelivery, refundPayment, cancelOrder,
	}, test.calls)
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
