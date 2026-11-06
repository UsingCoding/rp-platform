package migrations

import (
	"context"

	"github.com/distributed-programming-2026/go-sdk/pkg/migrator"
	"github.com/distributed-programming-2026/go-sdk/pkg/mysql"
)

func Version1793990856(client mysql.ClientContext) migrator.Migration {
	return &version1793990856{
		client: client,
	}
}

type version1793990856 struct {
	client mysql.ClientContext
}

func (v *version1793990856) Version() int64 {
	return 1793990856
}

func (v *version1793990856) Description() string {
	return "Create deliveries table"
}

func (v *version1793990856) Up(ctx context.Context) error {
	_, err := v.client.ExecContext(ctx, "create-deliveries-table", `
CREATE TABLE IF NOT EXISTS deliveries (
    order_id BINARY(16) NOT NULL PRIMARY KEY,
    address VARCHAR(1024) NOT NULL,
    created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6)
) ENGINE=InnoDB CHARACTER SET=utf8mb4 COLLATE=utf8mb4_unicode_ci
`)
	return err
}
