package controllers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"os"
	"strings"
	"time"
	"zeroshare-backend/structs"

	"gorm.io/gorm"
)

type Response struct {
	Nwid string `json:"nwid"`
}

func CreateNewZTNetwork(ctx context.Context, name string) (string, error) {
	nodeId := os.Getenv("ZU_NODE_ID")
	url := fmt.Sprintf("https://zero-controller.jkbx.live/controller/network/%s______", nodeId)

	log.Printf("URL: %s", url)

	// Create an HTTP client with a timeout
	client := &http.Client{Timeout: 10 * time.Second}

	jsonData, _ := json.Marshal(generateNetworkConfig(name))
	log.Printf("JSON Data: %s", jsonData)
	bodyReader := bytes.NewReader(jsonData)

	// Create a request with the given context
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bodyReader)
	req.Header.Add("X-ZT1-AUTH", os.Getenv("ZU_TOKEN"))

	if err != nil {
		return "", fmt.Errorf("failed to create request: %v", err)
	}

	// Send the request
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to make request: %v", err)
	}
	defer resp.Body.Close()

	// Check for a successful status code
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	// Decode the JSON response
	var result Response
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("failed to decode response: %v", err)
	}

	return result.Nwid, nil
}

func FetchAllMembers(ctx context.Context, networkId string, userId string, db *gorm.DB) ([]Member, error) {
	var peers []structs.Peer
	if err := db.Where("network_id = ? AND user_id = ?", networkId, userId).Find(&peers).Error; err != nil {
		return nil, err
	}
	var members []Member
	for _, peer := range peers {
		member, err := getMemberDetails(ctx, networkId, peer.NodeId)
		if err != nil {
			return nil, err
		}
		members = append(members, member)
	}
	return members, nil
}

func getMemberDetails(ctx context.Context, networkId string, nodeId string) (Member, error) {
	url := fmt.Sprintf("https://zero-controller.jkbx.live/controller/network/%s/member/%s", networkId, nodeId)
	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	req.Header.Add("X-ZT1-AUTH", os.Getenv("ZU_TOKEN"))
	if err != nil {
		return Member{}, fmt.Errorf("failed to create request: %v", err)	
	}

	resp, err := client.Do(req)
	if err != nil {
		return Member{}, fmt.Errorf("failed to make request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return Member{}, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var peer Member
	if err := json.NewDecoder(resp.Body).Decode(&peer); err != nil {
		return Member{}, fmt.Errorf("failed to decode response: %v", err)
	}

	return peer, nil
}

type Route struct {
	Target string       `json:"target"`
	Via    *interface{} `json:"via"`
	Flags  int          `json:"flags"`
	Metric int          `json:"metric"`
}

type IpAssignmentPool struct {
	IpRangeStart string `json:"ipRangeStart"`
	IpRangeEnd   string `json:"ipRangeEnd"`
}

type NetworkConfig struct {
	Name              string             `json:"name"`
	Private           bool               `json:"private"`
	V6AssignMode      map[string]bool    `json:"v6AssignMode"`
	V4AssignMode      map[string]bool    `json:"v4AssignMode"`
	Routes            []Route            `json:"routes"`
	IpAssignmentPools []IpAssignmentPool `json:"ipAssignmentPools"`
	EnableBroadcast   bool               `json:"enableBroadcast"`
}

type Member struct {
	ActiveBridge                bool     `json:"activeBridge"`
	Address                     string   `json:"address"`
	AuthenticationExpiryTime    int64    `json:"authenticationExpiryTime"`
	Authorized                  bool     `json:"authorized"`
	Capabilities                []string `json:"capabilities"`
	CreationTime                int64    `json:"creationTime"`
	ID                          string   `json:"id"`
	Identity                    string   `json:"identity"`
	IPAssignments               []string `json:"ipAssignments"`
	LastAuthorizedCredential    *string  `json:"lastAuthorizedCredential"`
	LastAuthorizedCredentialType string   `json:"lastAuthorizedCredentialType"`
	LastAuthorizedTime          int64    `json:"lastAuthorizedTime"`
	LastDeauthorizedTime        int64    `json:"lastDeauthorizedTime"`
	Name                        string   `json:"name"`
	NoAutoAssignIps             bool     `json:"noAutoAssignIps"`
	Nwid                        string   `json:"nwid"`
	Objtype                     string   `json:"objtype"`
	RemoteTraceLevel            int      `json:"remoteTraceLevel"`
	RemoteTraceTarget           *string  `json:"remoteTraceTarget"`
	Revision                    int      `json:"revision"`
	SsoExempt                   bool     `json:"ssoExempt"`
	Tags                        []string `json:"tags"`
	VMajor                      int      `json:"vMajor"`
	VMinor                      int      `json:"vMinor"`
	VProto                      int      `json:"vProto"`
	VRev                        int      `json:"vRev"`
}

func getRandomInt(min, max int) int {
	return rand.Intn(max-min) + min
}

func generateNetworkConfig(name string) NetworkConfig {
	randSubnetPart := getRandomInt(0, 254)

	return NetworkConfig{
		Name:         name,
		Private:      true,
		V6AssignMode: map[string]bool{"rfc4193": false, "6plane": false, "zt": false},
		V4AssignMode: map[string]bool{"zt": true},
		Routes: []Route{
			{
				Target: fmt.Sprintf("172.30.%d.0/24", randSubnetPart),
				Via:    nil,
				Flags:  0,
				Metric: 0,
			},
		},
		IpAssignmentPools: []IpAssignmentPool{
			{
				IpRangeStart: fmt.Sprintf("172.30.%d.1", randSubnetPart),
				IpRangeEnd:   fmt.Sprintf("172.30.%d.254", randSubnetPart),
			},
		},
		EnableBroadcast: true,
	}
}

func AddPeerAndAuthorize(ctx context.Context, peer structs.Peer, DB *gorm.DB) error {
	url := fmt.Sprintf("https://zero-controller.jkbx.live/controller/network/%s/member/%s", peer.NetworkId, peer.NodeId)
	client := &http.Client{Timeout: 10 * time.Second}

	jsonStr := fmt.Sprintf(`{"name": "%s", "authorized": true}`, peer.MachineName)
	reader := strings.NewReader(jsonStr)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, reader)
	req.Header.Add("X-ZT1-AUTH", os.Getenv("ZU_TOKEN"))

	if err != nil {
		return fmt.Errorf("failed to create request: %v", err)
	}

	// Send the request
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to make request: %v", err)

	}
	defer resp.Body.Close()

	// Check for a successful status code
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	DB.Where(structs.Peer{NodeId: peer.NodeId, NetworkId: peer.NetworkId}).FirstOrCreate(&peer)

	return nil
}
