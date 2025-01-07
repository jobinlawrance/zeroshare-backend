package controllers

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/redis/go-redis/v9"
)

func SetupRedis() *redis.Client {
	redisHost := os.Getenv("REDIS_HOST")
	redisPort := os.Getenv("REDIS_PORT")
	client := redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%s", redisHost, redisPort),
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