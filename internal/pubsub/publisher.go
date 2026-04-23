package pubsub

import (
	"context"

	"cloud.google.com/go/pubsub/v2"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

type Publisher struct {
	topic *pubsub.Publisher
}

func NewPublisher(ctx context.Context, projectID string, topicID string) *Publisher {
	client, _ := pubsub.NewClient(ctx, projectID)
	return &Publisher{topic: client.Publisher(topicID)}
}

func (p *Publisher) Publish(ctx context.Context, msg proto.Message) error {
	data, err := protojson.Marshal(msg)
	if err != nil {
		return err
	}

	result := p.topic.Publish(ctx, &pubsub.Message{Data: data})
	_, err = result.Get(ctx)

	return err
}
