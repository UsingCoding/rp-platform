package migrations

import (
	"context"

	"github.com/distributed-programming-2026/go-sdk/pkg/migrator"
	"github.com/distributed-programming-2026/go-sdk/pkg/mysql"
)

func Version1793992575(client mysql.ClientContext) migrator.Migration {
	return &version1793992575{
		client: client,
	}
}

type version1793992575 struct {
	client mysql.ClientContext
}

func (v *version1793992575) Version() int64 {
	return 1793992575
}

func (v *version1793992575) Description() string {
	return "Add status to deliveries"
}

func (v *version1793992575) Up(ctx context.Context) error {
	_, err := v.client.ExecContext(ctx, "add-deliveries-status", `
ALTER TABLE deliveries ADD COLUMN status VARCHAR(16) NOT NULL DEFAULT 'scheduled'
`)
	return err
}
