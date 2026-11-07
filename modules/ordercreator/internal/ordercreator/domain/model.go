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
	Chaos   Chaos
}

// Chaos lists service methods that must fail or be delayed; used for chaos testing.
type Chaos struct {
	MethodError string
	MethodSleep []string
}
