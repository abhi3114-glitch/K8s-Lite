package deployment

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/abhigod/k8s-lite/internal/api"
	"github.com/abhigod/k8s-lite/internal/client"
)

type Controller struct {
	Client *client.Client
}

func New(client *client.Client) *Controller {
	return &Controller{Client: client}
}

func (c *Controller) Start(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := c.reconcile(ctx); err != nil {
				log.Printf("Error reconciling deployments: %v", err)
			}
		}
	}
}

func (c *Controller) reconcile(ctx context.Context) error {
	deployments, err := c.Client.ListDeployments(ctx)
	if err != nil {
		return err
	}

	// For each deployment, ensure we have the correct RS
	for _, d := range deployments {
		if err := c.syncDeployment(ctx, &d); err != nil {
			log.Printf("Error syncing deployment %s: %v", d.Name, err)
		}
	}
	return nil
}

func (c *Controller) syncDeployment(ctx context.Context, d *api.Deployment) error {
	// 1. List all ReplicaSets
	allRS, err := c.Client.ListReplicaSets(ctx)
	if err != nil {
		return err
	}

	// 2. Filter RS owned by this Deployment (by Label Selector for now, or Name convention)
	// For MVP, we use labels match.
	var ownedRS []*api.ReplicaSet
	for i := range allRS {
		rs := &allRS[i]
		if labelsMatch(d.Spec.Selector.MatchLabels, rs.ObjectMeta.Labels) {
			ownedRS = append(ownedRS, rs)
		}
	}

	// 3. Compute Hash of current PodTemplateSpec
	// This identifies the "New" RS we want.
	podTemplateHash := computeHash(d.Spec.Template)

	// 4. Find the New RS (if exists)
	var newRS *api.ReplicaSet
	for _, rs := range ownedRS {
		if rs.ObjectMeta.Annotations["deployment.kubernetes.io/revision"] == podTemplateHash {
			newRS = rs
			break
		}
	}

	// 5. If New RS doesn't exist, Create it
	if newRS == nil {
		log.Printf("Creating new ReplicaSet for Deployment %s (hash: %s)", d.Name, podTemplateHash)
		newRS, err = c.createNewReplicaSet(ctx, d, podTemplateHash)
		if err != nil {
			return fmt.Errorf("failed to create new RS: %v", err)
		}
		// Refresh ownedRS list?? Or just append
		ownedRS = append(ownedRS, newRS)
	}

	// 6. Rolling Update Logic
	// We want to scale UP newRS to d.Spec.Replicas
	// And scale DOWN old RSs to 0.
	// MVP: Simple "Recreate-ish" or fast rolling.
	// Ensure NewRS.Spec.Replicas == Deployment.Spec.Replicas

	desiredReplicas := int32(1)
	if d.Spec.Replicas != nil {
		desiredReplicas = *d.Spec.Replicas
	}

	// Identify Old RS
	var oldRS []*api.ReplicaSet
	currentReplicas := int32(0)
	for _, rs := range ownedRS {
		if rs.Name != newRS.Name {
			oldRS = append(oldRS, rs)
			if rs.Spec.Replicas != nil {
				currentReplicas += *rs.Spec.Replicas
			}
		}
	}

	// Naively:
	// 1. Scale Up New RS to Desired (eventually)
	// 2. Scale Down Old RS (eventually)

	// If we just set New=Desired, and Old=0, Controller Manager will do it parallel.
	// Real K8s uses MaxSurge/MaxUnavailable logic.
	// Let's implement a simple step:
	// If New < Desired, New++
	// If Total > Desired + Surge, Old--

	// MVP Simplified:
	// Just Set NewRS.Replicas = Desired.
	// Set OldRS.Replicas = 0.
	// This is basically "Surge to 200%" if we don't wait.
	// If we want actual rolling, we should verify NewRS has Ready pods before scaling down Old.

	// Let's do:
	// Ensure NewRS matches Desired.
	if newRS.Spec.Replicas == nil || *newRS.Spec.Replicas != desiredReplicas {
		// Update New RS
		replicas := desiredReplicas
		newRS.Spec.Replicas = &replicas
		// API Update RS? We don't have UpdateRS in client yet?
		// Wait, we need UpdateReplicaSet.
		// Oh, implementation_plan vs reality.
		// I need to add UpdateReplicaSet to client too.
		// Assuming I will add it or have it.
		// Actually I don't think I added UpdateReplicaSet in client.go earlier steps.
		// I only added ListReplicaSets and CreateReplicaSet.
		// I need UpdateReplicaSet.
		// Let's assume I will add        log.Printf("Scaling New RS %s to %d", newRS.Name, replicas)
		log.Printf("Scaling New RS %s to %d", newRS.Name, replicas)
		if err := c.Client.UpdateReplicaSet(ctx, newRS); err != nil {
			log.Printf("Failed to update RS %s: %v", newRS.Name, err)
		}
	}

	// Scale down Old
	for _, old := range oldRS {
		if old.Spec.Replicas != nil && *old.Spec.Replicas > 0 {
			zero := int32(0)
			old.Spec.Replicas = &zero
			log.Printf("Scaling Down Old RS %s to 0", old.Name)
			if err := c.Client.UpdateReplicaSet(ctx, old); err != nil {
				log.Printf("Failed to update RS %s: %v", old.Name, err)
			}
		}
	}

	return nil
}

func (c *Controller) createNewReplicaSet(ctx context.Context, d *api.Deployment, hash string) (*api.ReplicaSet, error) {
	replicas := int32(0) // Start at 0? Or Start at Desired if we are aggressive.
	// Let's start at 0 and let reconcile loop scale it up?
	// Or set to desired if it's the first one.
	// If we have old ones, maybe start at 0?
	// Let's set to Desired immediately for MVP "Surge"
	if d.Spec.Replicas != nil {
		replicas = *d.Spec.Replicas
	}

	rsName := fmt.Sprintf("%s-%s", d.Name, hash[:10])

	rs := &api.ReplicaSet{
		TypeMeta: api.TypeMeta{Kind: "ReplicaSet", APIVersion: "apps/v1"},
		ObjectMeta: api.ObjectMeta{
			Name:      rsName,
			Namespace: d.Namespace,
			Labels:    d.Spec.Template.Labels, // Match pod labels
			Annotations: map[string]string{
				"deployment.kubernetes.io/revision": hash,
			},
			// OwnerReferences?
		},
		Spec: api.ReplicaSetSpec{
			Replicas: &replicas,
			Selector: d.Spec.Selector,
			Template: d.Spec.Template,
		},
	}
	// Ensure selector matches template labels for RS to work
	if rs.Spec.Selector.MatchLabels == nil {
		rs.Spec.Selector.MatchLabels = make(map[string]string)
	}
	// Copy labels?

	if err := c.Client.CreateReplicaSet(ctx, rs); err != nil {
		return nil, err
	}
	return rs, nil
}

func computeHash(template api.PodTemplateSpec) string {
	data, _ := json.Marshal(template)
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:])
}

func labelsMatch(selector, labels map[string]string) bool {
	for k, v := range selector {
		if labels[k] != v {
			return false
		}
	}
	return true
}





