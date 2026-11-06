package migrations

import (
	"context"

	"github.com/distributed-programming-2026/go-sdk/pkg/migrator"
	"github.com/distributed-programming-2026/go-sdk/pkg/mysql"
)

func Version1793990855(client mysql.ClientContext) migrator.Migration {
	return &version1793990855{
		client: client,
	}
}

type version1793990855 struct {
	client mysql.ClientContext
}

func (v *version1793990855) Version() int64 {
	return 1793990855
}

func (v *version1793990855) Description() string {
	return "Create orders table"
}

func (v *version1793990855) Up(ctx context.Context) error {
	_, err := v.client.ExecContext(ctx, "create-orders-table", `
CREATE TABLE IF NOT EXISTS orders (
    order_id BINARY(16) NOT NULL PRIMARY KEY,
    amount BIGINT NOT NULL,
    address VARCHAR(1024) NOT NULL,
    created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6)
) ENGINE=InnoDB CHARACTER SET=utf8mb4 COLLATE=utf8mb4_unicode_ci
`)
	return err
}
