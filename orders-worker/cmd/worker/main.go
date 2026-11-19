package main

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/acme/orders-worker/internal/queue"
)

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(log)
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	q, err := queue.New(ctx)
	if err != nil {
		log.Error("queue init failed", "err", err)
		os.Exit(1)
	}

	log.Info("orders-worker consuming")
	if err := q.Run(ctx, handle); err != nil && !errors.Is(err, context.Canceled) {
		log.Error("worker stopped", "err", err)
		os.Exit(1)
	}
	log.Info("orders-worker stopped")
}

// handle is where your business logic goes. It must be idempotent: every
// supported broker delivers at least once.
func handle(_ context.Context, m queue.Message) error {
	if m.Body == "" {
		return errors.New("empty message body")
	}
	slog.Info("processed message", "id", m.ID, "bytes", len(m.Body))
	return nil
}
