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
	ID      uuid.UUID
	Amount  int64
	Address string
	// MethodError names the service method that must fail; used for chaos testing.
	MethodError string
}
