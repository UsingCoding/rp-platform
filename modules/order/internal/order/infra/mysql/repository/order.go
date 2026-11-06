package repository

import (
	"context"

	"github.com/distributed-programming-2026/go-sdk/pkg/mysql"

	"order/internal/order/domain"
)

func NewOrderRepository(ctx context.Context, client mysql.ClientContext) domain.OrderRepository {
	return &orderRepository{
		ctx:    ctx,
		client: client,
	}
}

type orderRepository struct {
	ctx    context.Context
	client mysql.ClientContext
}

// Store ignores repeated calls for the same order: callers retry on failure.
func (r *orderRepository) Store(o domain.Order) error {
	_, err := r.client.ExecContext(r.ctx, "orders.store", `
INSERT INTO orders (order_id, amount, address) VALUES (?, ?, ?)
ON DUPLICATE KEY UPDATE order_id = order_id
`, o.OrderID, o.Amount, o.Address)
	return err
}
