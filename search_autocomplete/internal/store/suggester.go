package store

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"

	"github.com/arjun118/autocomplete/internal/suggest"
)

type Suggester struct {
	memory *MemoryCache
	redis  *RedisCache
	mongo  *MongoStore
}

func NewSuggester() (*Suggester, error) {
	memory := NewMemoryCache()

	redisAddr := os.Getenv("REDIS_ADDR")
	if redisAddr == "" {
		redisAddr = "localhost:6379"
	}
	redis, err := NewRedisCache(redisAddr)
	if err != nil {
		return nil, fmt.Errorf("redis init error: %w", err)
	}

	mongoURI := os.Getenv("MONGO_URI")
	if mongoURI == "" {
		mongoURI = "mongodb://localhost:27017"
	}
	mongo := NewMongoStore(mongoURI)

	return &Suggester{
		memory: memory,
		redis:  redis,
		mongo:  mongo,
	}, nil
}

func (s *Suggester) Init(ctx context.Context, memUnits int, redisUnits int) error {
	if memUnits >= redisUnits {
		return errors.New("memUnits must be strictly less than redisUnits")
	}

	records, err := s.mongo.GetTopPrefixes(redisUnits)
	if err != nil {
		return fmt.Errorf("failed to load seed prefixes from mongo: %w", err)
	}
	log.Printf("[SEED] fetched %d records from mongodb\n", len(records))
	pipe := s.redis.client.Pipeline()
	const batchSize = 5000

	for i, record := range records {
		if i < memUnits {
			s.memory.Set(record.Prefix, record.Top)
		}
		if err := s.redis.SetPipelined(ctx, pipe, record.Prefix, record.Top); err != nil {
			log.Printf("[SEED WARNING] pipeline queue error for prefix %q: %v", record.Prefix, err)
		}
		if (i+1)%batchSize == 0 || i == len(records)-1 {
			if _, err := pipe.Exec(ctx); err != nil {
				log.Printf("[SEED WARNING] pipeline flush error at batch %d: %v", i/batchSize, err)
			}
		}
	}

	log.Printf("[SEED] records loaded in-memory: %d\n", memUnits)
	return nil
}

func (s *Suggester) Suggest(prefix string) (suggestions []string, source string) {
	var result []suggest.Reco
	if res, ok := s.memory.Get(prefix); ok {
		result = res
		source = "in-memory"
	} else if res, ok := s.redis.Get(prefix); ok {
		result = res
		source = "cache"
	} else {
		dbRes, err := s.mongo.Get(prefix)
		if err != nil {
			return nil, ""
		}
		result = dbRes
		source = "db"
		go func(p string, recs []suggest.Reco) {
			_ = s.redis.Set(p, recs)
		}(prefix, result)
	}

	suggestions = make([]string, len(result))
	for i, r := range result {
		suggestions[i] = r.Word
	}
	return suggestions, source
}
