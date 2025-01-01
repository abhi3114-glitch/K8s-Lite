package service

import (
	"context"
	"log"
	"reflect"
	"sort"
	"time"

	"github.com/abhigod/k8s-lite/internal/api"
	"github.com/abhigod/k8s-lite/internal/client"
)

type ServiceController struct {
	Client *client.Client
}

func NewController(client *client.Client) *ServiceController {
	return &ServiceController{
		Client: client,
	}
}

func (c *ServiceController) Run(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	log.Println("Service Controller started")

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := c.syncServices(ctx); err != nil {
				log.Printf("Error syncing services: %v", err)
			}
		}
	}
}

func (c *ServiceController) syncServices(ctx context.Context) error {
	services, err := c.Client.ListServices(ctx)
	if err != nil {
		return err
	}

	for _, svc := range services {
		if err := c.reconcileService(ctx, &svc); err != nil {
			log.Printf("Error reconciling service %s: %v", svc.Name, err)
		}
	}
	return nil
}

func (c *ServiceController) reconcileService(ctx context.Context, svc *api.Service) error {
	// If the service has no selector, we don't manage endpoints (user might).
	if len(svc.Spec.Selector) == 0 {
		return nil
	}

	// 1. Find Pods matching selector
	// For MVP, we list all pods and filter. Inefficient but fine for now.
	// Assume single namespace or matching namespace.
	pods, err := c.Client.ListPods(ctx, "") // All pods
	if err != nil {
		return err
	}

	var matchingPods []api.Pod
	for _, pod := range pods {
		if pod.Namespace != svc.Namespace {
			continue // Should match namespace
		}
		if isMatch(pod.Labels, svc.Spec.Selector) {
			if pod.Status.Phase == "Running" && pod.Status.PodIP != "" {
				matchingPods = append(matchingPods, pod)
			}
		}
	}

	// 2. Build Desired Endpoints
	subset := api.EndpointSubset{}
	for _, pod := range matchingPods {
		subset.Addresses = append(subset.Addresses, api.EndpointAddress{
			IP:       pod.Status.PodIP,
			NodeName: pod.Spec.NodeName,
		})
	}
	// Sort to ensure stable order for comparison
	sort.Slice(subset.Addresses, func(i, j int) bool {
		return subset.Addresses[i].IP < subset.Addresses[j].IP
	})

	// Add ports from Service
	for _, sp := range svc.Spec.Ports {
		subset.Ports = append(subset.Ports, api.EndpointPort{
			Name:     sp.Name,
			Port:     sp.TargetPort.IntVal, // Using TargetPort (on pod)
			Protocol: sp.Protocol,
		})
		// Note: Protocol defaulting to TCP if empty?
		if sp.Protocol == "" {
			subset.Ports[len(subset.Ports)-1].Protocol = "TCP"
		}
		// If TargetPort is string/name, we'd need to lookup container port names.
		// MVP: assume Int.
		if sp.TargetPort.Type == 1 { // String
			// TODO: Implement named port lookup
			// For now, warn and skip? Or try to use Port?
			// Defaulting to Port if mapped 1:1?
			// Let's assume user provides int for TargetPort in MVP.
		} else if sp.TargetPort.IntVal == 0 {
			// If targetPort is not set, it defaults to Port
			subset.Ports[len(subset.Ports)-1].Port = sp.Port
		}
	}

	desiredEp := &api.Endpoints{
		ObjectMeta: api.ObjectMeta{
			Name:      svc.Name,
			Namespace: svc.Namespace,
			Labels:    svc.Labels,
		},
		Subsets: []api.EndpointSubset{},
	}
	if len(subset.Addresses) > 0 {
		desiredEp.Subsets = append(desiredEp.Subsets, subset)
	}

	// 3. Get Existing Endpoints
	existingEp, err := c.Client.GetEndpoints(ctx, svc.Name)
	if err != nil {
		return err
	}

	if existingEp == nil {
		// Create
		log.Printf("Creating Endpoints for Service %s with %d addresses", svc.Name, len(subset.Addresses))
		return c.Client.CreateEndpoints(ctx, desiredEp)
	}

	// 4. Update if different
	// DeepEqual check
	if !reflect.DeepEqual(existingEp.Subsets, desiredEp.Subsets) {
		existingEp.Subsets = desiredEp.Subsets
		log.Printf("Updating Endpoints for Service %s with %d addresses", svc.Name, len(subset.Addresses))
		return c.Client.UpdateEndpoints(ctx, existingEp)
	}

	return nil
}

func isMatch(labels, selector map[string]string) bool {
	for k, v := range selector {
		if labels[k] != v {
			return false
		}
	}
	return true
}



