package repository

import (
	"context"

	"github.com/distributed-programming-2026/go-sdk/pkg/mysql"

	"payment/internal/payment/domain"
)

func NewPaymentRepository(ctx context.Context, client mysql.ClientContext) domain.PaymentRepository {
	return &paymentRepository{
		ctx:    ctx,
		client: client,
	}
}

type paymentRepository struct {
	ctx    context.Context
	client mysql.ClientContext
}

// Store ignores repeated calls for the same order: callers retry on failure.
func (r *paymentRepository) Store(p domain.Payment) error {
	_, err := r.client.ExecContext(r.ctx, "payments.store", `
INSERT INTO payments (order_id, amount) VALUES (?, ?)
ON DUPLICATE KEY UPDATE order_id = order_id
`, p.OrderID, p.Amount)
	return err
}
