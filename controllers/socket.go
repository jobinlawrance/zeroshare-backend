package controllers

import (
	"context"
	"encoding/json"
	"log"
	structs "zeroshare-backend/structs"

	"github.com/gofiber/contrib/websocket"
	"github.com/redis/go-redis/v9"
)

func Stream(c *websocket.Conn, redisStore *redis.Client) {
	// Context for Redis operations
	ctx := context.Background()
	var device structs.Device
	if err := c.ReadJSON(&device); err != nil {
		log.Println("Error reading device info:", err)
		c.Close()
		return
	}

	deviceID := device.ID.String()
	log.Printf("Device connected: %s", deviceID)

	// Subscribe to Redis channel for the device
	subscriber := redisStore.Subscribe(context.Background(), deviceID)
	defer subscriber.Close()

	// Listen for messages on the Redis channel
	// Goroutine to handle incoming messages from Redis
	go func() {
		for msg := range subscriber.Channel() {
			var response structs.SSEResponse
			if err := json.Unmarshal([]byte(msg.Payload), &response); err != nil {
				log.Println("Error unmarshaling Redis message:", err)
				continue
			}

			// Send the message to the WebSocket client
			if err := c.WriteJSON(response); err != nil {
				log.Println("Error writing JSON to WebSocket:", err)
				break
			}
		}
	}()

	// Handle incoming messages from the WebSocket client
	for {
		var request structs.SSERequest
		if err := c.ReadJSON(&request); err != nil {
			log.Println("Error reading WebSocket message:", err)
			break
		}

		// Create SSEResponse to publish to Redis
		response := structs.SSEResponse{
			Type:   request.Type,
			Data:   request.Data,
			Device: device,
		}

		// Publish to Redis
		responseData, err := json.Marshal(response)
		if err != nil {
			log.Println("Error marshaling response:", err)
			continue
		}
		if err := redisStore.Publish(ctx, request.DeviceID, responseData).Err(); err != nil {
			log.Println("Error publishing to Redis:", err)
			continue
		}
		log.Printf("Published to Redis channel %s: %s", request.DeviceID, responseData)
	}
}