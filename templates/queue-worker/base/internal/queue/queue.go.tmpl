// Package queue hides the message broker behind one interface.
// The adapter in provider.go is the only target-specific code in this repo.
package queue

import "context"

// Message is one delivery from the queue.
type Message struct {
	ID   string
	Body string
}

// Handler processes a message. Returning an error leaves the message on the
// queue so the broker redelivers it and, after enough attempts, dead-letters it.
type Handler func(ctx context.Context, m Message) error

// Queue delivers messages to a handler until the context is cancelled.
type Queue interface {
	Run(ctx context.Context, h Handler) error
}
