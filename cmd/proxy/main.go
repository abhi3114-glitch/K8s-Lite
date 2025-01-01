package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/abhigod/k8s-lite/internal/client"
	"github.com/abhigod/k8s-lite/internal/proxy"
)

func main() {
	apiURL := flag.String("api-url", "http://localhost:8080", "URL of API Server")
	tlsCert := flag.String("tls-cert", "", "Path to client certificate")
	tlsKey := flag.String("tls-key", "", "Path to client key")
	tlsCA := flag.String("tls-ca", "", "Path to CA certificate")
	flag.Parse()

	log.Println("Starting Kube-Proxy...")

	cli := client.New(*apiURL, *tlsCert, *tlsKey, *tlsCA)
	proxier := proxy.NewProxier(cli)

	ctx, cancel := context.WithCancel(context.Background())
	go proxier.Run(ctx)

	// Wait for sigterm
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	<-c

	log.Println("Shutting down Kube-Proxy...")
	cancel()
}



