package pubsub

import (
	"context"

	"cloud.google.com/go/pubsub/v2"
)

type Publisher struct {
	topic *pubsub.Publisher
}

func NewPublisher(ctx context.Context, projectID string, topicID string) *Publisher {
	client, _ := pubsub.NewClient(ctx, projectID)
	return &Publisher{topic: client.Publisher(topicID)}
}

func (p *Publisher) Publish(ctx context.Context, data []byte) error {
	result := p.topic.Publish(ctx, &pubsub.Message{Data: data})
	_, err := result.Get(ctx)

	return err
}
