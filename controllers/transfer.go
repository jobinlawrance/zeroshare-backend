package controllers

import (
	"bufio"
	"context"
	"fmt"
	"log"

	"github.com/gofiber/fiber/v2"
	"github.com/redis/go-redis/v9"
	"github.com/valyala/fasthttp"
)

func DeviceSSE(c *fiber.Ctx, redisStore *redis.Client, deviceId string) error {

	c.Set("Content-Type", "text/event-stream")
	c.Set("Cache-Control", "no-cache")
	c.Set("Connection", "keep-alive")
	c.Set("Transfer-Encoding", "chunked")

	subscriber := redisStore.Subscribe(context.Background(), deviceId)

	// Listen for messages on the Redis channel and send them as SSE
	c.Status(fiber.StatusOK).Context().SetBodyStreamWriter(fasthttp.StreamWriter(func(w *bufio.Writer) {
		for {
			msg, err := subscriber.ReceiveMessage(context.Background())
			if err != nil {
				log.Printf("Error receiving message: %v", err)
				break
			}

			// Send the SSE formatted data
			log.Printf("Sending SSE event to client: %s", deviceId)
			data := fmt.Sprintf("data: %s\n\n", msg.Payload)

			log.Printf("Payload: %s", msg.Payload)

			// Write data to the stream
			if _, err := w.WriteString(data); err != nil {
				log.Printf("Error writing to stream: %v", err)
				break
			}

			// Flush the response to send the data immediately
			if err := w.Flush(); err != nil {
				log.Printf("Error flushing stream: %v", err)
				break
			}
			log.Println("Received message:", msg.Payload)
		}
	}))

	return nil
}