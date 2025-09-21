package main

import (
	"context"
	"encoding/json"
	"os"

	"github.com/redis/go-redis/v9"
)

type Redis struct {
	client *redis.Client
}

func NewRedis() *Redis {
	redisDsn, ok := os.LookupEnv("REDIS_DSN")
	if !ok {
		return nil
	}
	return &Redis{client: redis.NewClient(&redis.Options{Addr: redisDsn})}
}

func (r *Redis) PublishNotification(jobID string, status JobStatus) error {
	notification := &RedisNotification{
		JobID:  jobID,
		Status: status,
	}
	output, err := json.Marshal(notification)
	if err != nil {
		return err
	}
	err = r.client.Publish(context.TODO(), os.Getenv("REDIS_CHANNEL"), output).Err()
	if err != nil {
		return err
	}
	return nil
}
