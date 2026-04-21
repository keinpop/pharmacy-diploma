package kafka

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/segmentio/kafka-go"
	"go.uber.org/zap"

	usecase "pharmacy/inventory/domain/use_case"
)

// KafkaProducer реализует usecase.EventPublisher через segmentio/kafka-go.
type KafkaProducer struct {
	writer *kafka.Writer
	logger *zap.Logger
}

// NewKafkaProducer создаёт продюсер с подключением к брокерам Kafka.
// brokers — строка с адресами брокеров через запятую, например "kafka:9092".
func NewKafkaProducer(brokers string, logger *zap.Logger) *KafkaProducer {
	brokersList := strings.Split(brokers, ",")
	for i, b := range brokersList {
		brokersList[i] = strings.TrimSpace(b)
	}
	writer := &kafka.Writer{
		Addr:         kafka.TCP(brokersList...),
		Balancer:     &kafka.LeastBytes{},
		RequiredAcks: kafka.RequireOne,
	}
	return &KafkaProducer{writer: writer, logger: logger}
}

// PublishBatchReceived публикует событие получения партии в топик "inventory.received".
func (p *KafkaProducer) PublishBatchReceived(ctx context.Context, event usecase.BatchReceivedEvent) error {
	return p.publish(ctx, "inventory.received", event.ProductID, event)
}

// PublishBatchWrittenOff публикует событие списания партии в топик "inventory.written_off".
func (p *KafkaProducer) PublishBatchWrittenOff(ctx context.Context, event usecase.BatchWrittenOffEvent) error {
	return p.publish(ctx, "inventory.written_off", event.ProductID, event)
}

func (p *KafkaProducer) publish(ctx context.Context, topic, key string, payload interface{}) error {
	jsonBytes, err := json.Marshal(payload)
	if err != nil {
		p.logger.Error("kafka marshal error", zap.String("topic", topic), zap.Error(err))
		return err
	}
	msg := kafka.Message{
		Topic: topic,
		Key:   []byte(key),
		Value: jsonBytes,
	}
	if err := p.writer.WriteMessages(ctx, msg); err != nil {
		p.logger.Error("kafka write error", zap.String("topic", topic), zap.Error(err))
		return err
	}
	return nil
}

// Close закрывает соединение с Kafka.
func (p *KafkaProducer) Close() error {
	return p.writer.Close()
}

// keep time import used by event structs
var _ = time.Now
