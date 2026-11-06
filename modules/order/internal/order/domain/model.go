package domain

import "uuid"

type Echo struct {
	ID   uuid.UUID
	Body string
}

type EchoRepository interface {
	NextID() (uuid.UUID, error)
	Store(Echo) error
}

type Order struct {
	OrderID uuid.UUID
	Amount  int64
	Address string
}

type OrderRepository interface {
	Store(Order) error
}
