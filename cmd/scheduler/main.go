package main

import (
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/abhigod/k8s-lite/internal/scheduler"
)

func main() {
	apiURL := flag.String("api-url", "http://localhost:8080", "URL of API Server")
	flag.Parse()

	sched := scheduler.New(*apiURL)

	go sched.Start()

	// Wait for sigterm
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	<-c
	log.Println("Shutting down Scheduler...")
}

