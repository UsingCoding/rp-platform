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

type Payment struct {
	OrderID uuid.UUID
	Amount  int64
}

type PaymentRepository interface {
	Store(Payment) error
}
