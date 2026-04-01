package database

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/redis/go-redis/v9"
)

func InitRedis() *redis.Client {
	redisURL := os.Getenv("REDIS_URL")

	var opts *redis.Options
	var err error

	opts, err = redis.ParseURL(redisURL)
	if err != nil {
		log.Fatalf("Failed to parse Redis URL: %v", err)
	}
	rdb := redis.NewClient(opts)

	_, err = rdb.Ping(context.Background()).Result()
	if err != nil {
		log.Fatalf("Failed to connect to Redis: %v", err)
	}

	return rdb
}

func AddToBlackList(ctx context.Context, rdb *redis.Client, token string, expiration time.Duration) error {
	key := fmt.Sprintf("blacklist:%s", token)

	return rdb.Set(ctx, key, "revoked", expiration).Err()
}

func IsTokenBlackListed(ctx context.Context, rdb *redis.Client, token string) (bool, error) {
	key := fmt.Sprintf("blacklist:%s", token)

	exists, err := rdb.Exists(ctx, key).Result()
	if err != nil {
		return false, err
	}

	return exists > 0, nil
}
