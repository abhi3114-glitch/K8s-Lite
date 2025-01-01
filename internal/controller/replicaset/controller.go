package replicaset

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/abhigod/k8s-lite/internal/api"
	"github.com/abhigod/k8s-lite/internal/client"
	"github.com/google/uuid"
)

type Controller struct {
	Client *client.Client
}

func New(client *client.Client) *Controller {
	return &Controller{
		Client: client,
	}
}

func (c *Controller) Start(ctx context.Context) {
	log.Println("Starting ReplicaSet Controller...")
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			c.reconcile(ctx)
		}
	}
}

func (c *Controller) reconcile(ctx context.Context) {
	// 1. List All ReplicaSets
	rss, err := c.Client.ListReplicaSets(ctx)
	if err != nil {
		log.Printf("Error listing replicasets: %v", err)
		return
	}

	// 2. List All Pods (Optimization: List once and filter in memory)
	// 2. List All Pods (Optimization: List once and filter in memory)
	pods, err := c.Client.ListPods(ctx, "")
	if err != nil {
		log.Printf("Error listing pods: %v", err)
		return
	}

	// 3. Reconcile each RS
	for _, rs := range rss {
		c.reconcileRS(ctx, &rs, pods)
	}
}

func (c *Controller) reconcileRS(ctx context.Context, rs *api.ReplicaSet, allPods []api.Pod) {
	if rs.Spec.Replicas == nil {
		return
	}
	desired := *rs.Spec.Replicas

	// Find owned pods
	// Simple label match
	var ownedPods []api.Pod
	for _, pod := range allPods {
		if labelsMatch(rs.Spec.Selector.MatchLabels, pod.Labels) {
			ownedPods = append(ownedPods, pod)
		}
	}

	current := int32(len(ownedPods))
	log.Printf("RS %s: Desired=%d, Current=%d", rs.Name, desired, current)

	if current < desired {
		// Scale Up
		diff := desired - current
		log.Printf("Scaling up RS %s by %d", rs.Name, diff)
		for i := int32(0); i < diff; i++ {
			if err := c.createPod(ctx, rs); err != nil {
				log.Printf("Failed to create pod for RS %s: %v", rs.Name, err)
			}
		}
	} else if current > desired {
		// Scale Down
		diff := current - desired
		log.Printf("Scaling down RS %s by %d", rs.Name, diff)
		// Delete simplest ones (e.g. Pending, or just random first ones)
		// We'll delete from the end of our list
		for i := int32(0); i < diff; i++ {
			pod := ownedPods[i] // Simple selection
			if err := c.Client.DeletePod(ctx, pod.Name); err != nil {
				log.Printf("Failed to delete pod %s: %v", pod.Name, err)
			}
		}
	}
}

func (c *Controller) createPod(ctx context.Context, rs *api.ReplicaSet) error {
	// Create Pod based on template
	pod := &api.Pod{
		TypeMeta: api.TypeMeta{
			Kind:       "Pod",
			APIVersion: "v1",
		},
		ObjectMeta: rs.Spec.Template.ObjectMeta,
		Spec:       rs.Spec.Template.Spec,
	}

	// Generate Name: rs-name-random
	pod.Name = fmt.Sprintf("%s-%s", rs.Name, uuid.New().String()[:5])

	// Ensure labels are set from template (they should match selector)
	if pod.Labels == nil {
		pod.Labels = make(map[string]string)
	}
	// Copy RS labels too? No, template labels.

	// Owner Ref? Not implemented yet.

	return c.Client.CreatePod(ctx, pod)
}

func labelsMatch(selector map[string]string, labels map[string]string) bool {
	if len(selector) == 0 {
		return false // Empty selector matches nothing? Or everything? Kubernetes says empty selector matches everything usually, but for RS it should be strict.
		// Let's assume matches nothing to be safe against accidental adoption.
	}
	for k, v := range selector {
		if val, ok := labels[k]; !ok || val != v {
			return false
		}
	}
	return true
}





