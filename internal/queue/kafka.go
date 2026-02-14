package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"million-rps/internal/config"
	"million-rps/internal/models"
	"million-rps/pkg/logger"

	"github.com/segmentio/kafka-go"
)

// EnsureTopic creates the todo-commands topic with configured partitions (idempotent).
// Call at startup; if it fails (e.g. no broker or topic exists), app still runs.
func EnsureTopic(ctx context.Context) {
	cfg := config.Get()
	if len(cfg.KafkaBrokers) == 0 {
		return
	}
	conn, err := kafka.Dial("tcp", cfg.KafkaBrokers[0])
	if err != nil {
		logger.Debug(ctx, "Kafka dial for topic creation failed", "error", err)
		return
	}
	defer conn.Close()
	controller, err := conn.Controller()
	if err != nil {
		logger.Debug(ctx, "Kafka controller lookup failed", "error", err)
		return
	}
	ctrlConn, err := kafka.Dial("tcp", fmt.Sprintf("%s:%d", controller.Host, controller.Port))
	if err != nil {
		logger.Debug(ctx, "Kafka controller dial failed", "error", err)
		return
	}
	defer ctrlConn.Close()
	err = ctrlConn.CreateTopics(kafka.TopicConfig{
		Topic:             cfg.KafkaTopic,
		NumPartitions:     cfg.KafkaPartitions,
		ReplicationFactor: 1,
	})
	if err != nil {
		logger.Debug(ctx, "Kafka create topic failed (topic may already exist)", "error", err)
		return
	}
	logger.Info(ctx, "Kafka topic ensured", "topic", cfg.KafkaTopic, "partitions", cfg.KafkaPartitions)
}

var (
	writer *kafka.Writer
	wOnce  sync.Once
)

// Producer returns the global Kafka writer for todo commands (initialized on first use).
func Producer(ctx context.Context) *kafka.Writer {
	wOnce.Do(func() {
		cfg := config.Get()
		writer = &kafka.Writer{
			Addr:         kafka.TCP(cfg.KafkaBrokers...),
			Topic:        cfg.KafkaTopic,
			Balancer:     &kafka.LeastBytes{},
			BatchSize:    100,
			BatchTimeout: 0,
			Async:        true,
			RequiredAcks: kafka.RequireOne,
		}
		logger.Info(ctx, "Kafka producer initialized", "topic", cfg.KafkaTopic, "brokers", cfg.KafkaBrokers)
	})
	return writer
}

// PublishTodoCommand publishes a todo command to Kafka. Non-blocking when using Async writer.
func PublishTodoCommand(ctx context.Context, cmd *models.TodoCommand) error {
	w := Producer(ctx)
	if w == nil {
		return nil
	}
	payload, err := json.Marshal(cmd)
	if err != nil {
		return err
	}
	key := []byte(cmd.UserID + ":" + cmd.Action)
	return w.WriteMessages(ctx, kafka.Message{
		Key:   key,
		Value: payload,
	})
}

// Topic returns the todo commands topic name.
func Topic() string {
	return config.Get().KafkaTopic
}

// Brokers returns Kafka broker addresses.
func Brokers() []string {
	return config.Get().KafkaBrokers
}
