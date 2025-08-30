package kafka

import (
	"context"

	"github.com/alex-pvl/go-tapmenu-consumer/internal/app/config"
	"github.com/segmentio/kafka-go"
)

type Consumer struct {
	reader *kafka.Reader
}

func NewConsumer(configuration *config.Configuration) *Consumer {
	readerConfig := kafka.ReaderConfig{
		Brokers:        []string{configuration.KafkaAddress},
		Topic:          configuration.TopicName,
		GroupID:        configuration.ConsumerGroup,
		CommitInterval: 0, // no auto-commit
	}

	reader := kafka.NewReader(readerConfig)

	return &Consumer{reader: reader}
}

func (c *Consumer) Consume(ctx context.Context) (msg kafka.Message, err error) {
	msg, err = c.reader.ReadMessage(ctx)
	err = c.reader.CommitMessages(ctx, msg)
	return
}
