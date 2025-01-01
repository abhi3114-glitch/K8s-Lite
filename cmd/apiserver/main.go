package main

import (
	"flag"
	"log"
	"net/http"
	"os"

	"github.com/abhigod/k8s-lite/internal/apiserver"
	"github.com/abhigod/k8s-lite/internal/storage"
)

func main() {
	var dataFile string
	flag.StringVar(&dataFile, "data-file", "k8s-lite.db", "Path to data file for persistence")

	var tlsCert, tlsKey, tlsCA string
	flag.StringVar(&tlsCert, "tls-cert", "", "Path to server certificate")
	flag.StringVar(&tlsKey, "tls-key", "", "Path to server key")
	flag.StringVar(&tlsCA, "tls-ca", "", "Path to CA certificate for client auth")

	flag.Parse()

	log.Println("Starting K8s-Lite API Server...")

	// 1. Initialize Storage (File-backed)
	store := storage.NewMemoryStore(dataFile)

	// 2. Initialize API Server
	server := apiserver.NewServer(store)

	// 3. Start HTTP Server
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("Listening on port %s", port)

	if tlsCert != "" && tlsKey != "" && tlsCA != "" {
		log.Println("Serving with TLS (mTLS enabled)...")
		if err := server.ServeTLS(":"+port, tlsCert, tlsKey, tlsCA); err != nil {
			log.Fatalf("Server failed: %v", err)
		}
	} else {
		log.Println("Serving insecurely (HTTP)...")
		if err := http.ListenAndServe(":"+port, server.Router); err != nil {
			log.Fatalf("Server failed: %v", err)
		}
	}
}





