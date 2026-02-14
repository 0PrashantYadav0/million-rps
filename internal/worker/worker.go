package worker

import (
	"context"
	"encoding/json"
	"sync/atomic"

	"million-rps/internal/cache"
	"million-rps/internal/config"
	"million-rps/internal/models"
	"million-rps/internal/queue"
	"million-rps/internal/repository"
	"million-rps/pkg/logger"

	"github.com/segmentio/kafka-go"
)

// Run starts the Kafka consumer: reads todo commands, applies to DB, invalidates cache.
// One consumer per process; scale by running more replicas (consumer group shares partitions).
func Run(ctx context.Context) {
	cfg := config.Get()
	if len(cfg.KafkaBrokers) == 0 {
		logger.Info(ctx, "Worker disabled (no Kafka brokers)")
		return
	}
	topic := queue.Topic()
	brokers := queue.Brokers()
	if len(brokers) == 0 {
		return
	}

	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:  brokers,
		Topic:    topic,
		GroupID:  "todo-workers",
		MinBytes: 1,
		MaxBytes: 10e6,
	})
	defer reader.Close()

	var processed int64
	logger.Info(ctx, "Kafka consumer started", "topic", topic)
	for {
		msg, err := reader.FetchMessage(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			logger.Error(ctx, "Worker fetch failed", "error", err)
			continue
		}
		if err := handleMessage(ctx, msg.Value); err != nil {
			logger.Error(ctx, "Worker handle failed", "error", err, "payload", string(msg.Value))
			// Commit anyway to avoid poison pill blocking the partition
			_ = reader.CommitMessages(ctx, msg)
			continue
		}
		if err := reader.CommitMessages(ctx, msg); err != nil {
			logger.Error(ctx, "Worker commit failed", "error", err)
		}
		atomic.AddInt64(&processed, 1)
	}
}

func handleMessage(ctx context.Context, payload []byte) error {
	var cmd models.TodoCommand
	if err := json.Unmarshal(payload, &cmd); err != nil {
		return err
	}
	switch cmd.Action {
	case "create":
		todo := &models.Todo{
			ID:          cmd.ID,
			Title:       cmd.Title,
			Description: cmd.Description,
			UserID:      cmd.UserID,
		}
		if cmd.Completed != nil {
			todo.Completed = *cmd.Completed
		}
		if err := repository.Create(ctx, todo); err != nil {
			return err
		}
	case "update":
		if err := repository.Update(ctx, cmd.ID, cmd.UserID, cmd.Title, cmd.Description, cmd.Completed); err != nil {
			return err
		}
	case "delete":
		if err := repository.Delete(ctx, cmd.ID, cmd.UserID); err != nil {
			return err
		}
	default:
		return nil
	}
	cache.InvalidateTodos(ctx)
	return nil
}
