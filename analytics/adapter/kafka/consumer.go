package kafka

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/segmentio/kafka-go"
	"go.uber.org/zap"

	chchadapter "pharmacy/analytics/adapter/clickhouse"
	"pharmacy/analytics/app/metrics"
)

// StartConsumers launches three Kafka consumer goroutines:
// one each for sales.completed, inventory.written_off, and inventory.received.
func StartConsumers(ctx context.Context, brokers string, eventRepo *chchadapter.EventRepo, logger *zap.Logger) {
	brokerList := strings.Split(brokers, ",")

	go consumeSales(ctx, brokerList, eventRepo, logger)
	go consumeWriteOffs(ctx, brokerList, eventRepo, logger)
	go consumeReceived(ctx, brokerList, eventRepo, logger)
}

func consumeSales(ctx context.Context, brokers []string, eventRepo *chchadapter.EventRepo, logger *zap.Logger) {
	topic := "sales.completed"
	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers: brokers,
		Topic:   topic,
		GroupID: "analytics-" + topic,
	})
	defer reader.Close()

	for {
		msg, err := reader.ReadMessage(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			logger.Warn("sales consumer read error", zap.Error(err))
			continue
		}

		var event chchadapter.SaleEvent
		if err := json.Unmarshal(msg.Value, &event); err != nil {
			logger.Warn("sales consumer unmarshal error", zap.Error(err), zap.ByteString("value", msg.Value))
			continue
		}

		if err := eventRepo.InsertSaleEvent(ctx, event); err != nil {
			logger.Warn("sales consumer insert error", zap.Error(err))
			continue
		}
		metrics.KafkaEventsConsumed.WithLabelValues(topic).Inc()
	}
}

func consumeWriteOffs(ctx context.Context, brokers []string, eventRepo *chchadapter.EventRepo, logger *zap.Logger) {
	topic := "inventory.written_off"
	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers: brokers,
		Topic:   topic,
		GroupID: "analytics-" + topic,
	})
	defer reader.Close()

	for {
		msg, err := reader.ReadMessage(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			logger.Warn("write-off consumer read error", zap.Error(err))
			continue
		}

		var event chchadapter.WriteOffEvent
		if err := json.Unmarshal(msg.Value, &event); err != nil {
			logger.Warn("write-off consumer unmarshal error", zap.Error(err), zap.ByteString("value", msg.Value))
			continue
		}

		if err := eventRepo.InsertWriteOffEvent(ctx, event); err != nil {
			logger.Warn("write-off consumer insert error", zap.Error(err))
			continue
		}
		metrics.KafkaEventsConsumed.WithLabelValues(topic).Inc()
	}
}

func consumeReceived(ctx context.Context, brokers []string, eventRepo *chchadapter.EventRepo, logger *zap.Logger) {
	topic := "inventory.received"
	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers: brokers,
		Topic:   topic,
		GroupID: "analytics-" + topic,
	})
	defer reader.Close()

	for {
		msg, err := reader.ReadMessage(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			logger.Warn("received consumer read error", zap.Error(err))
			continue
		}

		var event chchadapter.ReceivedEvent
		if err := json.Unmarshal(msg.Value, &event); err != nil {
			logger.Warn("received consumer unmarshal error", zap.Error(err), zap.ByteString("value", msg.Value))
			continue
		}

		if err := eventRepo.InsertReceivedEvent(ctx, event); err != nil {
			logger.Warn("received consumer insert error", zap.Error(err))
			continue
		}
		metrics.KafkaEventsConsumed.WithLabelValues(topic).Inc()
	}
}
