package main

import "github.com/redis/go-redis/v9"

type RedisClient struct {
}

func NewRedisClient() *redis.Client {
	return redis.NewClient(&redis.Options{
		Addr:             "localhost:6389",
		Password:         "",
		DB:               0,
		DisableIndentity: true,
	})
}
