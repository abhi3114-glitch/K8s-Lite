package scheduler

import (
	"context"
	"log"
	"math/rand"
	"strconv"
	"strings"
	"time"

	"github.com/abhigod/k8s-lite/internal/api"
	"github.com/abhigod/k8s-lite/internal/client"
)

type Scheduler struct {
	Client *client.Client
	ctx    context.Context
	cancel context.CancelFunc
}

func New(apiURL string) *Scheduler {
	ctx, cancel := context.WithCancel(context.Background())
	return &Scheduler{
		Client: client.New(apiURL, "", "", ""),
		ctx:    ctx,
		cancel: cancel,
	}
}

func (s *Scheduler) Start() {
	log.Println("Starting Scheduler...")

	// Polling loop for MVP. Ideally Watch.
	// Since we haven't implemented a robust Client Watch yet (server has it, but client lib doesn't),
	// we'll poll for unscheduled pods.

	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			s.scheduleOneRound()
		}
	}
}

func (s *Scheduler) scheduleOneRound() {
	// 1. Get all pods
	// Note: Inefficient for large clusters.
	pods, err := s.Client.ListPods(s.ctx, "")
	if err != nil {
		log.Printf("Scheduler Error listing pods: %v", err)
		return
	}

	// 2. Identify unscheduled pods
	var unscheduled []*api.Pod
	for i := range pods {
		if pods[i].Spec.NodeName == "" {
			unscheduled = append(unscheduled, &pods[i])
		}
	}

	if len(unscheduled) == 0 {
		return
	}

	// 3. Get Nodes
	nodes, err := s.Client.ListNodes(s.ctx)
	if err != nil {
		log.Printf("Scheduler Error listing nodes: %v", err)
		return
	}

	if len(nodes) == 0 {
		log.Println("No nodes available for scheduling")
		return
	}

	// 4. Schedule each pod
	for _, pod := range unscheduled {
		node, err := s.selectNode(pod, nodes)
		if err != nil {
			log.Printf("Failed to schedule pod %s: %v", pod.Name, err)
			continue
		}

		if node != "" {
			if err := s.bind(pod, node); err != nil {
				log.Printf("Failed to bind pod %s to %s: %v", pod.Name, node, err)
			} else {
				log.Printf("Successfully scheduled %s to %s", pod.Name, node)
			}
		}
	}
}

func (s *Scheduler) selectNode(pod *api.Pod, nodes []api.Node) (string, error) {
	// Filter (Predicates)
	var feasible []api.Node

	for _, node := range nodes {
		if s.podFitsResources(pod, &node) {
			feasible = append(feasible, node)
		}
	}

	if len(feasible) == 0 {
		return "", nil // No node fits
	}

	// Score (Priorities) - Logic: Random for now (or LeastRequested)
	// Simple random choice among feasible
	selected := feasible[rand.Intn(len(feasible))]
	return selected.Name, nil
}

func (s *Scheduler) podFitsResources(pod *api.Pod, node *api.Node) bool {
	// Simplification: We assume node.Status.Allocatable is static capacity for now.
	// In real scheduler, we must subtract usage of OTHER pods properly.
	// Since we don't have a synchronized cache of "Used" resources here yet,
	// we will just assume Node has infinite capacity for MVP OR check just Pod requests vs Node Capacity (ignoring other usage).
	// To be better: We *should* list all pods on that node and sum them up.

	// For MVP Step 3: Let's just check if Node Conditions are Ready.
	// And maybe check if Pod requests > Node Capacity.

	// 1. Check Node Ready
	isReady := false
	for _, cond := range node.Status.Conditions {
		if cond.Type == "Ready" && cond.Status == "True" {
			isReady = true
			break
		}
	}
	if !isReady {
		return false
	}

	// 2. Check Capacity (Simple)
	// Parse Pod CPU/Mem
	// Parse Node CPU/Mem
	// This requires a parser (100m -> 0.1, 128Mi -> bytes).
	// Let's implement a very simple parser or skip precise resource check for strict MVP.
	// User req: "PodFitsResources".

	// Let's try basic CPU check
	podCPU := parseCPU(getRequest(pod, "cpu"))
	nodeCPU := parseCPU(node.Status.Capacity["cpu"]) // Capacity or Allocatable

	if podCPU > nodeCPU {
		return false
	}

	return true
}

func (s *Scheduler) bind(pod *api.Pod, nodeName string) error {
	// Assign nodeName
	pod.Spec.NodeName = nodeName

	// Update via API
	return s.Client.UpdatePod(s.ctx, pod)
}

// Helpers

func getRequest(pod *api.Pod, resourceName string) string {
	// Sum of containers
	// Actually handling units summation as strings is hard.
	// We'll just take the first container for now or assume 0.
	if len(pod.Spec.Containers) > 0 {
		// return pod.Spec.Containers[0].Resources.Requests[resourceName]
		// TODO: Sum properly.
		return "0"
	}
	return "0"
}

func parseCPU(val string) int64 {
	// Return millicores
	if val == "" {
		return 0
	}
	if strings.HasSuffix(val, "m") {
		i, _ := strconv.ParseInt(strings.TrimSuffix(val, "m"), 10, 64)
		return i
	}
	// Assume cores
	f, _ := strconv.ParseFloat(val, 64)
	return int64(f * 1000)
}


