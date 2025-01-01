package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/abhigod/k8s-lite/internal/client"
	"github.com/abhigod/k8s-lite/internal/controller/deployment"
	"github.com/abhigod/k8s-lite/internal/controller/replicaset"
	"github.com/abhigod/k8s-lite/internal/controller/service"
	"github.com/abhigod/k8s-lite/internal/leaderelection"
)

func main() {
	apiURL := flag.String("api-url", "http://localhost:8080", "URL of API Server")
	tlsCert := flag.String("tls-cert", "", "Path to client certificate")
	tlsKey := flag.String("tls-key", "", "Path to client key")
	tlsCA := flag.String("tls-ca", "", "Path to CA certificate")
	leaderElect := flag.Bool("leader-elect", false, "Enable leader election")
	flag.Parse()

	cli := client.New(*apiURL, *tlsCert, *tlsKey, *tlsCA)

	runControllers := func(ctx context.Context) {
		// ReplicaSet Controller
		rsController := replicaset.New(cli)
		go rsController.Start(ctx)

		// Deployment Controller
		depController := deployment.New(cli)
		go depController.Start(ctx)

		// Service Controller
		svcController := service.NewController(cli)
		go svcController.Run(ctx)

		log.Println("Controllers started")
		<-ctx.Done()
	}

	// Handle signals for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		log.Println("Shutting down Controller Manager...")
		cancel()
	}()

	if *leaderElect {
		// Default identity usually cached hostname + uuid
		host, _ := os.Hostname()
		identity := fmt.Sprintf("%s-%s-%d", host, os.Getenv("COMPUTERNAME"), os.Getpid())

		leaderelection.RunOrDie(ctx, leaderelection.Config{
			LockName:      "k8s-lite-controller-manager",
			Identity:      identity,
			LeaseDuration: 15 * time.Second,
			RenewDeadline: 10 * time.Second,
			RetryPeriod:   2 * time.Second,
			Client:        cli,
			Callbacks: leaderelection.Callbacks{
				OnStartedLeading: func(ctx context.Context) {
					runControllers(ctx)
				},
				OnStoppedLeading: func() {
					log.Fatalf("Lost leadership, restarting...")
				},
			},
		})
	} else {
		runControllers(ctx)
	}
}






