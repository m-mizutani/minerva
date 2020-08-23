package main

import (
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/m-mizutani/minerva/pkg/lambda"
	"github.com/m-mizutani/minerva/pkg/models"
	"github.com/pkg/errors"
)

var logger = lambda.Logger

func main() {
	lambda.StartHandler(Handler)
}

// Handler is main procedure of dispatcher
func Handler(args lambda.HandlerArguments) error {
	var event events.DynamoDBEvent
	if err := args.BindEvent(&event); err != nil {
		return err
	}

	now := time.Now().UTC()
	chunkService := args.ChunkService()
	sqsService := args.SQSService()

	var chunks []*models.Chunk
	if len(event.Records) > 0 {
		for _, record := range event.Records {
			if record.EventName != "MODIFY" && record.EventName != "INSERT" {
				continue
			}
			if record.Change.NewImage == nil {
				continue
			}

			chunk, err := models.NewChunkFromDynamoEvent(record.Change.NewImage)
			if err != nil {
				logger.WithField("record", record).Error("NewChunkFromDynamoEvent")
				return errors.Wrap(err, "Failed to parse record.Change.NewImage")
			}

			if chunkService.IsMergableChunk(chunk, now) {
				chunks = append(chunks, chunk)
			}
		}
	} else {
		idxChunks, err := chunkService.GetMergableChunks("index", now)
		if err != nil {
			return errors.Wrap(err, "Failed GetMergableChunks")
		}
		msgChunks, err := chunkService.GetMergableChunks("message", now)
		if err != nil {
			return errors.Wrap(err, "Failed GetMergableChunks")
		}
		chunks = append(chunks, idxChunks...)
		chunks = append(chunks, msgChunks...)
	}

	logger.WithField("chunks", chunks).Info("waiwai")

	for _, old := range chunks {
		chunk, err := chunkService.FreezeChunk(old)
		if chunk == nil {
			continue // The chunk is no longer avaiable
		}
		if err != nil {
			return errors.Wrap(err, "chunkService.FreezeChunk")
		}

		src, err := chunk.ToS3ObjectSlice()
		if err != nil {
			return errors.Wrap(err, "Failed ToS3ObjectSlice")
		}

		s3Key := models.BuildMergedS3ObjectKey(args.S3Prefix, chunk.Schema, chunk.Partition, chunk.ChunkKey)
		dst := models.NewS3Object(args.S3Region, args.S3Bucket, s3Key)
		q := models.MergeQueue{
			Schema:     models.ParquetSchemaName(chunk.Schema),
			SrcObjects: src,
			DstObject:  dst,
		}

		if err := sqsService.SendSQS(q, args.MergeQueueURL); err != nil {
			logger.WithField("queue", q).Error("internal.SendSQS")
			return errors.Wrap(err, "Failed SendSQS")
		}

		if _, err := chunkService.DeleteChunk(chunk); err != nil {
			logger.WithField("chunk", chunk).WithError(err).Error("DeleteChunk")
			return errors.Wrap(err, "Failed chunkService.DeleteChunk")
		}
	}

	return nil
}
