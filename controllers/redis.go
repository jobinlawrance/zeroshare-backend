package controllers

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/redis/go-redis/extra/redisotel/v9"
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
	// Enable tracing instrumentation.
	if err = redisotel.InstrumentTracing(client); err != nil {
		panic(err)
	}

	// Enable metrics instrumentation.
	if err = redisotel.InstrumentMetrics(client); err != nil {
		panic(err)
	}

	return client
}
