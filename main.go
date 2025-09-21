package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/rs/zerolog/log"
)

func main() {
	s3Config, err := config.LoadDefaultConfig(
		context.TODO(),
		config.WithRegion(os.Getenv("S3_REGION")),
		config.WithBaseEndpoint(os.Getenv("S3_ENDPOINT")),
		config.WithCredentialsProvider(credentials.StaticCredentialsProvider{
			Value: aws.Credentials{
				AccessKeyID:     os.Getenv("S3_ACCESS_KEY"),
				SecretAccessKey: os.Getenv("S3_SECRET_KEY"),
			},
		}),
	)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to load S3 config")
	}
	s3Client := s3.NewFromConfig(s3Config, func(o *s3.Options) { o.UsePathStyle = true })
	redis := NewRedis()

	log.Info().Msg("using ffmpeg backend")
	backend := FfmpegBackend{}
	if !backend.isAvailable() {
		log.Fatal().Msg("ffmpeg backend is not available")
	}

	conn, err := amqp.Dial(os.Getenv("RABBITMQ_URL"))
	if err != nil {
		log.Fatal().Err(err).Msg("failed to connect to RabbitMQ")
	}
	defer conn.Close()

	channel, err := conn.Channel()
	if err != nil {
		log.Fatal().Err(err).Msg("failed to open a channel")
	}
	defer channel.Close()
	channel.Qos(1, 0, false)

	messages, err := channel.Consume("transcode", "", false, false, false, false, nil)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to consume the queue")
	}

	forever := make(chan bool)

	go func() {
		for message := range messages {
			var msg QueueMessage
			var logBuffer bytes.Buffer
			if err := json.Unmarshal(message.Body, &msg); err != nil {
				log.Error().Err(err).Msg("failed to unmarshal the message")
				message.Nack(false, true)
				continue
			}

			if redis != nil {
				if err := redis.PublishNotification(msg.JobID, JobStatusQueued); err != nil {
					log.Error().Err(err).Str("jobId", msg.JobID).Msg("failed to publish the notification")
					message.Nack(false, true)
					continue
				}
			}

			tempDirPath, err := os.MkdirTemp("", "transcode")
			if err != nil {
				log.Error().Err(err).Str("jobId", msg.JobID).Msg("failed to create the temp directory")
				message.Nack(false, true)
				continue
			}

			command, err := backend.buildCmd(msg.RawURL)
			if err != nil {
				log.Error().Err(err).Str("jobId", msg.JobID).Msg("failed to prepare the command")
				message.Nack(false, true)
				continue
			}

			if redis != nil {
				if err := redis.PublishNotification(msg.JobID, JobStatusTranscoding); err != nil {
					log.Error().Err(err).Str("jobId", msg.JobID).Msg("failed to publish the notification")
					message.Nack(false, true)
					continue
				}
			}

			command.Dir = tempDirPath
			backend.setupLogOutput(command, &logBuffer)
			if err := command.Run(); err != nil {
				log.Error().Err(err).Str("jobId", msg.JobID).Msg("failed to execute the command")
				message.Nack(false, true)
				continue
			}

			log.Info().Msg("successfully executed the command")
			if redis != nil {
				if err := redis.PublishNotification(msg.JobID, JobStatusUploading); err != nil {
					log.Error().Err(err).Str("jobId", msg.JobID).Msg("failed to publish the notification")
					message.Nack(false, true)
					continue
				}
			}

			entries, err := os.ReadDir(tempDirPath)
			if err != nil {
				log.Error().Err(err).Str("jobId", msg.JobID).Str("path", tempDirPath).Msg("failed to read the output directory")
				message.Nack(false, true)
				continue
			}

			log.Info().Int("entries", len(entries)).Msg("successfully read the output directory")

			for _, entry := range entries {
				filePath := filepath.Join(tempDirPath, entry.Name())
				file, err := os.Open(filePath)
				if err != nil {
					log.Error().Err(err).Str("jobId", msg.JobID).Str("path", filePath).Msg("failed to open the file")
					message.Nack(false, true)
					break
				}

				_, err = s3Client.PutObject(context.TODO(), &s3.PutObjectInput{
					Bucket: aws.String(os.Getenv("S3_BUCKET")),
					Key:    aws.String(fmt.Sprintf("%s/%s", msg.Key, entry.Name())),
					Body:   file,
					// TODO: caching and content type
				})
				if err != nil {
					log.Error().Err(err).Str("jobId", msg.JobID).Str("path", filePath).Msg("failed to upload the file")
					message.Nack(false, true)
					break
				}

				log.Info().Str("path", filePath).Msg("successfully uploaded the file")
				if err := file.Close(); err != nil {
					log.Error().Err(err).Str("jobId", msg.JobID).Str("path", filePath).Msg("failed to close the file")
				}
			}

			if err := message.Ack(false); err != nil {
				log.Error().Err(err).Str("jobId", msg.JobID).Msg("failed to ack the request")
			}
			if err := os.RemoveAll(tempDirPath); err != nil {
				log.Error().Err(err).Str("jobId", msg.JobID).Msg("failed to clean up the temp directory")
			}
			if redis != nil {
				if err := redis.PublishNotification(msg.JobID, JobStatusFinished); err != nil {
					log.Error().Err(err).Str("jobId", msg.JobID).Msg("failed to publish the notification")
					message.Nack(false, true)
					continue
				}
			}
		}
	}()

	<-forever
}
