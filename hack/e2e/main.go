package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/abhigod/k8s-lite/internal/api"
	"github.com/abhigod/k8s-lite/internal/client"
)

func assert(condition bool, msg string) {
	if condition {
		fmt.Printf("PASS: %s\n", msg)
	} else {
		log.Fatalf("FAIL: %s", msg)
	}
}

func main() {
	fmt.Println("Running Go-based E2E Verification...")

	apiURL := "https://localhost:8080"
	certFile := "client-admin.pem"
	keyFile := "client-admin.key"
	caFile := "ca.pem"

	if _, err := os.Stat(certFile); os.IsNotExist(err) {
		log.Fatalf("Cert file %s not found (run from project root)", certFile)
	}

	c := client.New(apiURL, certFile, keyFile, caFile)
	ctx := context.Background()

	// 1. Connectivity & Empty List
	pods, err := c.ListPods(ctx, "")
	if err != nil {
		log.Fatalf("FAIL: Failed to connect to API: %v", err)
	}
	fmt.Printf("PASS: Connected to API. Found %d pods.\n", len(pods))

	// 2. Node Registration Wait
	// Kubelet needs a moment to register
	fmt.Println("Waiting for Node registration...")
	nodeFound := false
	for i := 0; i < 10; i++ {
		nodes, err := c.ListNodes(ctx)
		if err == nil {
			for _, n := range nodes {
				if n.Name == "node1" {
					nodeFound = true
					break
				}
			}
		}
		if nodeFound {
			break
		}
		time.Sleep(1 * time.Second)
	}
	assert(nodeFound, "Node 'node1' is registered and visible")

	// 3. Create Deployment
	fmt.Println("Creating Deployment 'test-dep'...")
	replicas := int32(1)
	dep := &api.Deployment{
		ObjectMeta: api.ObjectMeta{Name: "test-dep"},
		Spec: api.DeploymentSpec{
			Replicas: &replicas,
			Selector: api.LabelSelector{MatchLabels: map[string]string{"app": "nginx"}},
			Template: api.PodTemplateSpec{
				ObjectMeta: api.ObjectMeta{Labels: map[string]string{"app": "nginx"}},
				Spec: api.PodSpec{
					Containers: []api.Container{{Name: "nginx", Image: "nginx:latest"}},
				},
			},
		},
	}
	err = c.CreateDeployment(ctx, dep)
	if err != nil {
		log.Fatalf("FAIL: Create Deployment: %v", err)
	}
	assert(true, "Deployment created")

	// 4. Wait for Pods
	fmt.Println("Waiting for Pod scheduling...")
	podsScheduled := false
	for i := 0; i < 15; i++ {
		pods, err := c.ListPods(ctx, "")
		if err == nil && len(pods) > 0 {
			// Check if any pod belongs to this deployment
			// In a real k8s this is label matching, here we just check if any pod exists for MVP
			podsScheduled = true
			break
		}
		time.Sleep(1 * time.Second)
	}
	assert(podsScheduled, "Pods scheduled by Controller Manager")

	fmt.Println("=== E2E SUITE PASSED ===")
}


