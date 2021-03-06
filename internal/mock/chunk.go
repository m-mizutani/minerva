package mock

import (
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/m-mizutani/minerva/internal/repository"
	"github.com/m-mizutani/minerva/pkg/models"
)

// ChunkMockDB is mock of ChunkDynamoDB
type ChunkMockDB struct {
	Data map[string]map[string]*models.Chunk
}

// NewChunkMockDB  is constructor of ChunkMockDB
func NewChunkMockDB() *ChunkMockDB {
	return &ChunkMockDB{
		Data: map[string]map[string]*models.Chunk{},
	}
}

// GetWritableChunks returns writable chunks for now (because chunks are not locked)
func (x *ChunkMockDB) GetWritableChunks(schema, partition string, writableTotalSize int64) ([]*models.Chunk, error) {
	pk := "chunk/" + schema
	dataMap, ok := x.Data[pk]
	if !ok {
		return nil, nil
	}

	var output []*models.Chunk
	for sk, chunk := range dataMap {
		if !strings.HasPrefix(sk, partition) {
			continue
		}

		if chunk.TotalSize < writableTotalSize && !chunk.Freezed {
			output = append(output, chunk)
		}
	}

	return output, nil
}

// GetMergableChunks returns mergable chunks exceeding freezedAt or minChunkSize
func (x *ChunkMockDB) GetMergableChunks(schema string, createdBefore time.Time, minChunkSize int64) ([]*models.Chunk, error) {
	pk := "chunk/" + schema
	dataMap, ok := x.Data[pk]
	if !ok {
		return nil, nil
	}

	var output []*models.Chunk
	for _, chunk := range dataMap {
		if minChunkSize <= chunk.TotalSize || chunk.CreatedAt <= createdBefore.UTC().Unix() {
			output = append(output, chunk)
		}
	}

	return output, nil
}

// PutChunk saves a new chunk into DB. The chunk must be overwritten by UUID.
func (x *ChunkMockDB) PutChunk(recordID string, objSize int64, schema, partition string, created time.Time) error {
	chunkKey := uuid.New().String()
	pk := "chunk/" + schema
	sk := partition + "/" + chunkKey

	chunk := &models.Chunk{
		PK: pk,
		SK: sk,

		Schema:    schema,
		Partition: partition,
		RecordIDs: []string{recordID},
		TotalSize: objSize,
		CreatedAt: created.Unix(),
		ChunkKey:  chunkKey,
		Freezed:   false,
	}

	pkMap, ok := x.Data[pk]
	if !ok {
		pkMap = map[string]*models.Chunk{}
		x.Data[pk] = pkMap
	}

	pkMap[sk] = chunk

	return nil
}

func (x *ChunkMockDB) UpdateChunk(chunk *models.Chunk, recordID string, objSize, writableSize int64) error {
	dataMap, ok := x.Data[chunk.PK]
	if !ok {
		return repository.ErrChunkNotWritable
	}
	target, ok := dataMap[chunk.SK]
	if !ok {
		return repository.ErrChunkNotWritable
	}

	// This statement is not in go manner. Because aligning to DynamoDB Filter condition
	if target.TotalSize < writableSize && !target.Freezed {
		target.TotalSize += objSize
		target.RecordIDs = append(target.RecordIDs, recordID)
	} else {
		return repository.ErrChunkNotWritable
	}

	return nil
}

func (x *ChunkMockDB) FreezeChunk(chunk *models.Chunk) (*models.Chunk, error) {
	dataMap, ok := x.Data[chunk.PK]
	if !ok {
		return nil, nil
	}
	target, ok := dataMap[chunk.SK]
	if !ok {
		return nil, nil
	}

	target.Freezed = true
	return target, nil
}

func (x *ChunkMockDB) DeleteChunk(chunk *models.Chunk) (*models.Chunk, error) {
	dataMap, ok := x.Data[chunk.PK]
	if !ok {
		return nil, nil
	}
	old, ok := dataMap[chunk.SK]
	if !ok {
		return nil, nil
	}

	delete(dataMap, chunk.SK)
	return old, nil
}
