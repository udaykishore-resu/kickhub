package queue

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
)

type sqsQueue struct {
	client *sqs.Client
	url    string
}

// New returns an SQS-backed Queue. Set AWS_ENDPOINT_URL to point the SDK at
// a local SQS-compatible broker; no code change is needed.
func New(ctx context.Context) (Queue, error) {
	url := os.Getenv("QUEUE_URL")
	if url == "" {
		return nil, errors.New("QUEUE_URL is not set")
	}
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return nil, err
	}
	return &sqsQueue{client: sqs.NewFromConfig(cfg), url: url}, nil
}

func (q *sqsQueue) Run(ctx context.Context, h Handler) error {
	for ctx.Err() == nil {
		out, err := q.client.ReceiveMessage(ctx, &sqs.ReceiveMessageInput{
			QueueUrl:            aws.String(q.url),
			MaxNumberOfMessages: 10,
			WaitTimeSeconds:     20, // long polling
		})
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			slog.Error("receive failed, retrying", "err", err)
			select {
			case <-ctx.Done():
			case <-time.After(2 * time.Second):
			}
			continue
		}
		for _, m := range out.Messages {
			msg := Message{ID: aws.ToString(m.MessageId), Body: aws.ToString(m.Body)}
			if err := h(ctx, msg); err != nil {
				// Not deleted: it becomes visible again and is redelivered.
				slog.Warn("handler failed", "id", msg.ID, "err", err)
				continue
			}
			if _, err := q.client.DeleteMessage(ctx, &sqs.DeleteMessageInput{
				QueueUrl: aws.String(q.url), ReceiptHandle: m.ReceiptHandle,
			}); err != nil {
				slog.Error("delete failed", "id", msg.ID, "err", err)
			}
		}
	}
	return nil
}
