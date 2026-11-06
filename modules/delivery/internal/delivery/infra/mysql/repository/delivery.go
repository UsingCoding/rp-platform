package repository

import (
	"context"

	"github.com/distributed-programming-2026/go-sdk/pkg/mysql"

	"delivery/internal/delivery/domain"
)

func NewDeliveryRepository(ctx context.Context, client mysql.ClientContext) domain.DeliveryRepository {
	return &deliveryRepository{
		ctx:    ctx,
		client: client,
	}
}

type deliveryRepository struct {
	ctx    context.Context
	client mysql.ClientContext
}

// Store ignores repeated calls for the same order: callers retry on failure.
func (r *deliveryRepository) Store(d domain.Delivery) error {
	_, err := r.client.ExecContext(r.ctx, "deliveries.store", `
INSERT INTO deliveries (order_id, address) VALUES (?, ?)
ON DUPLICATE KEY UPDATE order_id = order_id
`, d.OrderID, d.Address)
	return err
}
