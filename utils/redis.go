package utils

import (
	"context"
	"github.com/Romain-GUILLEMOT/WhispyrBack/config"
	"strconv"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

var Redis *redis.Client
var Ctx = context.Background()

func InitRedis() {
	cfg := config.GetConfig()
	db, _ := strconv.Atoi(cfg.RedisDB)

	Redis = redis.NewClient(&redis.Options{
		Addr:     cfg.RedisHost,
		Password: cfg.RedisPass,
		DB:       db,
	})

	if _, err := Redis.Ping(Ctx).Result(); err != nil {
		Fatal("Redis connection failed", "error", err)
	}

	Success("Redis connected successfully.")
}

func RedisSet(key, value string, ttl time.Duration, indexKeys ...string) error {
	pipe := Redis.TxPipeline()

	// Set principal
	pipe.Set(Ctx, key, value, ttl)

	// Si on t'a passé un index (ex: device_tokens:user:device)
	for _, indexKey := range indexKeys {
		pipe.SAdd(Ctx, indexKey, extractTokenFromKey(key))
		pipe.Expire(Ctx, indexKey, ttl)
	}

	_, err := pipe.Exec(Ctx)
	return err
}

func RedisGet(key string) (string, error) {
	return Redis.Get(Ctx, key).Result()
}

func RedisDel(key string) error {
	return Redis.Del(Ctx, key).Err()
}

func RedisExists(key string) (bool, error) {
	n, err := Redis.Exists(Ctx, key).Result()
	return n > 0, err
}
func RedisTTL(key string) (time.Duration, error) {
	ttl, err := Redis.TTL(Ctx, key).Result()
	if err != nil {
		return 0, err
	}
	return ttl, nil
}

// Nouvelle fonction: RedisSAdd ajoute des membres à un ensemble (set) Redis.
func RedisSAdd(ctx context.Context, key string, members ...interface{}) error {
	return Redis.SAdd(ctx, key, members...).Err()
}

// Nouvelle fonction: RedisSRem retire des membres d'un ensemble (set) Redis.
func RedisSRem(ctx context.Context, key string, members ...interface{}) error {
	return Redis.SRem(ctx, key, members...).Err()
}

// Nouvelle fonction: RedisPublish publie un message sur un canal Redis Pub/Sub.
func RedisPublish(ctx context.Context, channel string, message interface{}) error {
	return Redis.Publish(ctx, channel, message).Err()
}

// Nouvelle fonction: RedisPSubscribe s'abonne à un ou plusieurs canaux Redis Pub/Sub.
func RedisPSubscribe(ctx context.Context, channels ...string) *redis.PubSub {
	return Redis.PSubscribe(ctx, channels...)
}

// Nouvelle fonction: RedisSetWithTTL définit une clé avec une durée de vie.
func RedisSetWithTTL(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	return Redis.Set(ctx, key, value, ttl).Err()
}

func extractTokenFromKey(fullKey string) string {
	parts := strings.SplitN(fullKey, ":", 2)
	if len(parts) == 2 {
		return parts[1]
	}
	return fullKey
}
