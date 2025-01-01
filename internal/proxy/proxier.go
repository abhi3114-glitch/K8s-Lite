package proxy

import (
	"context"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net"
	"sync"
	"time"

	"github.com/abhigod/k8s-lite/internal/api"
	"github.com/abhigod/k8s-lite/internal/client"
)

type Proxier struct {
	Client *client.Client

	// Map <port> -> Listener
	listeners map[int32]net.Listener
	lock      sync.Mutex

	// Cache
	endpoints map[string]*api.Endpoints // ServiceName -> Endpoints
}

func NewProxier(client *client.Client) *Proxier {
	return &Proxier{
		Client:    client,
		listeners: make(map[int32]net.Listener),
		endpoints: make(map[string]*api.Endpoints),
	}
}

func (p *Proxier) Run(ctx context.Context) {
	log.Println("Starting K8s-Lite Proxy...")
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			p.closeAll()
			return
		case <-ticker.C:
			if err := p.sync(ctx); err != nil {
				log.Printf("Proxy Sync Error: %v", err)
			}
		}
	}
}

func (p *Proxier) sync(ctx context.Context) error {
	// 1. Update Endpoints Cache
	// In real K8s, we'd watch. Here we list all endpoints.
	// Actually, we iterate services to know what ports to open,
	// and we need endpoints for those services.

	services, err := p.Client.ListServices(ctx)
	if err != nil {
		return err
	}

	// Build map of desired NodePorts
	desiredPorts := make(map[int32]string) // port -> serviceName

	for _, svc := range services {
		for _, port := range svc.Spec.Ports {
			if port.NodePort != 0 {
				desiredPorts[port.NodePort] = svc.Name

				// Update cache for this service
				ep, err := p.Client.GetEndpoints(ctx, svc.Name)
				if err == nil && ep != nil {
					p.lock.Lock()
					p.endpoints[svc.Name] = ep
					p.lock.Unlock()
				}
			}
		}
	}

	p.lock.Lock()
	defer p.lock.Unlock()

	// 2. Open new listeners
	for port, svcName := range desiredPorts {
		if _, exists := p.listeners[port]; !exists {
			log.Printf("Opening Proxy Listener for Service %s on :%d", svcName, port)
			ln, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
			if err != nil {
				log.Printf("Failed to listen on :%d: %v", port, err)
				continue
			}
			p.listeners[port] = ln
			go p.serve(ln, svcName)
		}
	}

	// 3. Close old listeners
	for port, ln := range p.listeners {
		if _, desired := desiredPorts[port]; !desired {
			log.Printf("Closing Proxy Listener on :%d", port)
			ln.Close()
			delete(p.listeners, port)
		}
	}

	return nil
}

func (p *Proxier) serve(ln net.Listener, svcName string) {
	for {
		conn, err := ln.Accept()
		if err != nil {
			return // Listener closed
		}
		go p.handleConnection(conn, svcName)
	}
}

func (p *Proxier) handleConnection(inConn net.Conn, svcName string) {
	defer inConn.Close()

	// Pick backend
	p.lock.Lock()
	ep, ok := p.endpoints[svcName]
	p.lock.Unlock()

	if !ok || len(ep.Subsets) == 0 || len(ep.Subsets[0].Addresses) == 0 {
		log.Printf("No endpoints for %s, closing connection", svcName)
		return
	}

	// Simple random LB
	// Flatten addresses
	var backends []string
	subset := ep.Subsets[0]
	// Assume ports align? Mult-port support is tricky in this simple structure.
	// We just pick the first port from subset?
	// Or we match the incoming listener NodePort?
	// For MVP, assume 1 port per subset or random logic.
	targetPort := int32(80)
	if len(subset.Ports) > 0 {
		targetPort = subset.Ports[0].Port
	}

	for _, addr := range subset.Addresses {
		backends = append(backends, fmt.Sprintf("%s:%d", addr.IP, targetPort))
	}

	if len(backends) == 0 {
		return
	}

	backend := backends[rand.Intn(len(backends))]
	// log.Printf("Forwarding %s -> %s", svcName, backend)

	outConn, err := net.DialTimeout("tcp", backend, 2*time.Second)
	if err != nil {
		log.Printf("Dial failed to %s: %v", backend, err)
		return
	}
	defer outConn.Close()

	// Pipe
	go io.Copy(outConn, inConn) // Request
	io.Copy(inConn, outConn)    // Response
}

func (p *Proxier) closeAll() {
	p.lock.Lock()
	defer p.lock.Unlock()
	for _, ln := range p.listeners {
		ln.Close()
	}
}






