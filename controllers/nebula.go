package controllers

import (
	"context"
	"fmt"
	"log"
	"math/big"
	"net"
	"os"
	"os/exec"
	structs "zeroshare-backend/structs"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

func InitNebula(ctx context.Context) {
	// Paths to the CA certificate and key files
	caCrtPath := "ca.crt"
	caKeyPath := "ca.key"

	// Check if both files exist
	if !fileExists(caCrtPath) || !fileExists(caKeyPath) {
		// Prepare the command
		cmd := exec.Command("nebula-cert", "ca", "--name", "Jukebox, Inc")
		// Capture the output
		output, err := cmd.CombinedOutput()
		if err != nil {
			// Handle the error
			panic(err)
		}
		// Print the output
		println(string(output))
	}
}

func SignPublicKey(publicKey string, deviceId string, db *gorm.DB) (string, string, map[string]interface{}, error) {
	uid := uuid.New().String()
	fileName := fmt.Sprintf("%s.pub", uid)
	// Save the public key to the file
	if err := os.WriteFile(fileName, []byte(publicKey), 0644); err != nil {
		return "", "", map[string]interface{}{}, err
	}
	defer os.Remove(fileName)

	var latestDevice structs.Device
	err := db.Model(&structs.Device{}).Where("ip_address IS NOT NULL AND ip_address <> ''").Order("updated DESC").First(&latestDevice).Error

	if err != nil {
		// Handle error, e.g., log the error or return an appropriate error response
		log.Println("Error fetching latest device:", err)
		return "", "", map[string]interface{}{}, err
	}

	lastIp := latestDevice.IpAddress

	var device structs.Device
	err = db.Where("device_id = ?", deviceId).First(&device).Error
	if err != nil {
		return "", "", map[string]interface{}{}, err
	}

	newIP := ""
	if device.IpAddress == "" {
		newIP = incrementIP(lastIp)
	} else {
		newIP = device.IpAddress
	}
	log.Printf("New IP: %s", newIP)

	ipWithCIDR := fmt.Sprintf("%s/16", newIP)

	db.Where("device_id = ?", deviceId).Updates(structs.Device{IpAddress: newIP})

	certName := fmt.Sprintf("%s.neb.jkbx.live", uid)
	cmd := exec.Command("nebula-cert", "sign", "-in-pub", fileName, "-name", certName, "-ip", ipWithCIDR)
	_, err = cmd.CombinedOutput()
	if err != nil {
		return "", "", map[string]interface{}{}, err
	}
	// Read the generated certificate file
	certFile := fmt.Sprintf("%s.crt", certName)
	certContent, err := os.ReadFile(certFile)
	if err != nil {
		return "", "", map[string]interface{}{}, err
	}
	defer os.Remove(certFile)

	caCert, err := os.ReadFile("ca.crt")
	if err != nil {
		return "", "", map[string]interface{}{}, err
	}

	return string(certContent), string(caCert), getIncomingSite(uid), nil
}

func getIncomingSite(id string) map[string]interface{} {
	return map[string]interface{}{
		"name": id,
		"id":   id,
		"staticHostmap": map[string]interface{}{
			"69.69.0.1": map[string]interface{}{
				"lighthouse": true,
				"destinations": []string{
					"34.47.177.77:4242",
				},
			},
		},
		"unsafeRoutes": []string{},
		"ca":           "",
		"cert":         "",
		"key":          "",
		"lhDuration":   0,
		"port":         0,
		"mtu":          1300,
		"cipher":       "aes",
		"sortKey":      0,
		"logVerbosity": "info",
		"managed":      false,
		"rawConfig":    nil,
	}
}

// Helper function to check if a file exists
func fileExists(filename string) bool {
	info, err := os.Stat(filename)
	if os.IsNotExist(err) {
		return false
	}
	return !info.IsDir()
}

// Increment the given IP address
func incrementIP(lastIP string) string {
	ip := net.ParseIP(lastIP).To4()
	if ip == nil {
		return "69.69.0.2" // Default if lastIP is nil
	}

	// Convert IP to a big.Int, increment it, and convert it back
	ipInt := big.NewInt(0).SetBytes(ip)
	ipInt.Add(ipInt, big.NewInt(1))

	// Convert back to net.IP
	newIP := net.IP(ipInt.Bytes())
	return newIP.String()
}
