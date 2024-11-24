package controllers

import (
	"context"
	"fmt"
	"math/big"
	"net"
	"os"
	"os/exec"

	"github.com/google/uuid"
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

func SignPublicKey( publicKey string, lastIp string) (string, error) {
	uid := uuid.New().String()
	fileName := fmt.Sprintf("%s.pub", uid)
	// Save the public key to the file
	if err := os.WriteFile(fileName, []byte(publicKey), 0644); err != nil {
		return "", err
	}
	defer os.Remove(fileName)

	newIP := incrementIP(lastIp)
	fmt.Println(newIP)
	ipWithCIDR := fmt.Sprintf("%s/16", newIP)

	certName := fmt.Sprintf("%s.neb.jkbx.live", uid)
	cmd := exec.Command("nebula-cert", "sign", "-in-pub", fileName, "-name", certName, "-ip", ipWithCIDR)
	_, err := cmd.CombinedOutput()
	if err != nil {
		return "", err
	}
	// Read the generated certificate file
	certFile := fmt.Sprintf("%s.crt", certName)
	certContent, err := os.ReadFile(certFile)
	if err != nil {
		return "", err
	}
	defer os.Remove(certFile)

	return string(certContent), nil
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