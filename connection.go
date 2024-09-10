package cacheproxy

import (
	"context"
	"fmt"
	"log"

	"github.com/redis/go-redis/v9"
	"time"
)

type RedisClient struct {
	ctx context.Context
	c   *redis.Client
}

const CACHE_TTL = 120 * time.Second

var ctx context.Context

func InitRedisConnection(dns string) *RedisClient {
	ctx = context.Background()

	c := redis.NewClient(&redis.Options{
		Addr:             dns,
		Password:         "",
		DB:               0,
		DisableIndentity: true,
	})

	return &RedisClient{
		ctx: ctx,
		c:   c,
	}
}

func (r *RedisClient) Get(key string) (string, error) {
	val, err := r.c.Get(ctx, key).Result()

	if err == redis.Nil {
		return "", nil
	} else if err != nil {
		// Handle other Redis errors
		log.Printf("Error fetching key %s: %v", key, err)
		return "", err
	}

	return val, nil
}

func (r *RedisClient) Set(key string, v interface{}) {
	r.c.Set(ctx, key, v, CACHE_TTL)
}

func Test(rdb *redis.Client) {
	err := rdb.Set(ctx, "key", "value", 0).Err()
	if err != nil {
		panic(err)
	}

	val, err := rdb.Get(ctx, "key").Result()
	if err != nil {
		panic(err)
	}
	fmt.Println("key", val)

	val2, err := rdb.Get(ctx, "key2").Result()
	if err == redis.Nil {
		fmt.Println("key2 does not exist")
	} else if err != nil {
		panic(err)
	} else {
		fmt.Println("key2", val2)
	}
}
