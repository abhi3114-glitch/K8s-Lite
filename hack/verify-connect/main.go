package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/abhigod/k8s-lite/internal/client"
)

func main() {
	fmt.Println("Verifying K8s-Lite Connectivity...")

	apiURL := "https://localhost:8080"
	certFile := "client-admin.pem"
	keyFile := "client-admin.key"
	caFile := "ca.pem"

	// Verify files exist
	if _, err := os.Stat(certFile); os.IsNotExist(err) {
		log.Fatalf("Cert file %s not found", certFile)
	}

	c := client.New(apiURL, certFile, keyFile, caFile)

	pods, err := c.ListPods(context.Background(), "")
	if err != nil {
		log.Fatalf("FAIL: Failed to list pods: %v", err)
	}

	fmt.Printf("SUCCESS: Connected to API Server! Found %d pods.\n", len(pods))
}

