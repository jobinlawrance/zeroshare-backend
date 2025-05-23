package structs

import "encoding/json"

type SSERequest struct {
	Type     string          `json:"type"`
	Data     json.RawMessage `json:"data"`
	UniqueID string          `json:"uniqueId"`
	DeviceID string          `json:"deviceId"`
	SenderId string          `json:"senderId"`
}

type SSEResponse struct {
	Type   string          `json:"type"`
	Data   json.RawMessage `json:"data"`
	Device Device          `json:"device"`
}
