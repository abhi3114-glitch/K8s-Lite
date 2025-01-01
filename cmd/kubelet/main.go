package main

import (
	"flag"
	"log"
	"os"

	"github.com/abhigod/k8s-lite/internal/kubelet"
)

func main() {
	nodeName := flag.String("node-name", "", "Name of this node")
	apiURL := flag.String("api-url", "http://localhost:8080", "URL of API Server")
	tlsCert := flag.String("tls-cert", "", "Path to client certificate")
	tlsKey := flag.String("tls-key", "", "Path to client key")
	tlsCA := flag.String("tls-ca", "", "Path to CA certificate")
	flag.Parse()

	if *nodeName == "" {
		// Default to hostname
		host, _ := os.Hostname()
		nodeName = &host
	}

	agent := kubelet.NewAgent(*nodeName, *apiURL, *tlsCert, *tlsKey, *tlsCA)
	if err := agent.Start(); err != nil {
		log.Fatalf("Kubelet failed: %v", err)
	}
}






