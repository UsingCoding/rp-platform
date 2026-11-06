package app

import (
	"context"
	"uuid"

	"github.com/distributed-programming-2026/go-sdk/pkg/uow"

	domainevent "github.com/distributed-programming-2026/lib/event"

	"ordercreator/internal/ordercreator/domain"
	"ordercreator/internal/ordercreator/infra/mysql"
)

type DispatcherFactory interface {
	NewDispatcher(ctx context.Context) domainevent.Dispatcher
}

type OrderWorkflowStarter interface {
	StartCreateOrder(ctx context.Context, order domain.Order) (workflowID string, err error)
}

type Service struct {
	unit              uow.UnitOfWorkWithRepositoryProvider[*mysql.RepositoryProvider]
	dispatcherFactory DispatcherFactory
	orderStarter      OrderWorkflowStarter
}

func NewService(
	unit uow.UnitOfWorkWithRepositoryProvider[*mysql.RepositoryProvider],
	dispatcherFactory DispatcherFactory,
	orderStarter OrderWorkflowStarter,
) *Service {
	return &Service{
		unit:              unit,
		dispatcherFactory: dispatcherFactory,
		orderStarter:      orderStarter,
	}
}

func (s *Service) Echo(ctx context.Context, body string) (uuid.UUID, error) {
	var id uuid.UUID
	err := s.unit.ExecuteWithRepositoryProvider(ctx, func(provider *mysql.RepositoryProvider) error {
		svc := domain.NewEchoService(
			provider.Echo(ctx),
			s.dispatcherFactory.NewDispatcher(ctx),
		)

		var err error
		id, err = svc.Echo(body)
		return err
	})
	if err != nil {
		return uuid.Nil(), err
	}
	return id, nil
}

// CreateOrder starts the order creation workflow; it completes asynchronously.
func (s *Service) CreateOrder(ctx context.Context, amount int64, address, methodError string) (domain.Order, string, error) {
	order := domain.Order{
		ID:          uuid.New(),
		Amount:      amount,
		Address:     address,
		MethodError: methodError,
	}
	workflowID, err := s.orderStarter.StartCreateOrder(ctx, order)
	if err != nil {
		return domain.Order{}, "", err
	}
	return order, workflowID, nil
}
