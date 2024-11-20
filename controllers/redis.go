package controllers

import (
	"context"
	"log"

	"github.com/redis/go-redis/v9"
)

func SetupRedis() *redis.Client {
	client := redis.NewClient(&redis.Options{
		Addr:     "localhost:6379",
		Password: "testpass", // no password set
		DB:       0,          // use default DB
	})

	_, err := client.Ping(context.Background()).Result()
	if err != nil {
		log.Fatal(err)
		return nil
	}
	return client
}