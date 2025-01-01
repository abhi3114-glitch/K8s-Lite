package kubelet

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/abhigod/k8s-lite/internal/api"
	"github.com/abhigod/k8s-lite/internal/client"
)

type Agent struct {
	NodeName string
	Client   *client.Client
	Runtime  Runtime
	Prober   Prober

	ctx    context.Context
	cancel context.CancelFunc
}

func NewAgent(nodeName string, apiURL, tlsCert, tlsKey, tlsCA string) *Agent {
	ctx, cancel := context.WithCancel(context.Background())
	return &Agent{
		NodeName: nodeName,
		Client:   client.New(apiURL, tlsCert, tlsKey, tlsCA),
		Runtime:  NewDockerRuntime(),
		Prober:   NewProber(),
		ctx:      ctx,
		cancel:   cancel,
	}
}

func (a *Agent) Start() error {
	log.Printf("Starting Kubelet on node: %s", a.NodeName)

	// 1. Register Node
	if err := a.registerNode(); err != nil {
		return err
	}

	// 2. Start Sync Loop
	go a.syncLoop()

	<-a.ctx.Done()
	return nil
}

func (a *Agent) registerNode() error {
	node := &api.Node{
		ObjectMeta: api.ObjectMeta{
			Name: a.NodeName,
			Labels: map[string]string{
				"kubernetes.io/hostname": a.NodeName,
			},
		},
		Status: api.NodeStatus{
			Conditions: []api.NodeCondition{
				{Type: "Ready", Status: "True", LastHeartbeatTime: time.Now()},
			},
		},
	}

	log.Printf("Registering node %s...", a.NodeName)
	return a.Client.RegisterNode(a.ctx, node)
}

func (a *Agent) syncLoop() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-a.ctx.Done():
			return
		case <-ticker.C:
			a.runSync()
		}
	}
}

func (a *Agent) runSync() {
	// 1. Get Desired State (Pods from API)
	pods, err := a.Client.ListPods(a.ctx, a.NodeName)
	if err != nil {
		log.Printf("Error listing pods: %v", err)
		return
	}

	// Filter for my node
	var myPods []api.Pod
	for _, p := range pods {
		if p.Spec.NodeName == a.NodeName {
			myPods = append(myPods, p)
		}
	}

	// 2. Get Actual State (Containers from Runtime)
	containers, err := a.Runtime.ListContainers(a.ctx)
	if err != nil {
		log.Printf("Error listing containers: %v", err)
		return
	}
	// Debug log
	log.Printf("Found %d containers from runtime", len(containers))

	// Map of running pods (podName -> []ContainerInfo)
	runningPods := make(map[string][]ContainerInfo)
	for _, c := range containers {
		runningPods[c.PodName] = append(runningPods[c.PodName], c)
	}

	// 3. Reconcile

	// A. Create/Start missing pods
	for _, pod := range myPods {
		a.reconcilePod(&pod, runningPods[pod.Name])
		delete(runningPods, pod.Name) // Mark as handled
	}

	// B. Delete extra pods (that are no longer assigned to us)
	for podName, containers := range runningPods {
		// Only if it looks like a k8s pod (has podName)
		if podName != "" {
			log.Printf("Pod %s no longer assigned, cleaning up %d containers", podName, len(containers))
			for _, c := range containers {
				a.Runtime.StopContainer(a.ctx, c.ID, 0) // Force kill/immediate for cleanup
			}
		}
	}
}

func (a *Agent) reconcilePod(pod *api.Pod, runningContainers []ContainerInfo) {
	// Status Logic:
	// If all containers running -> Running
	// (Simply update phase for now)

	// We can assume Running if we find the container.
	// If not found, Pending?

	for _, specContainer := range pod.Spec.Containers {
		found := false
		for _, rc := range runningContainers {
			expectedName := fmt.Sprintf("k8s-lite-%s-%s", pod.Name, specContainer.Name)
			// log.Printf("Checking %s vs %s", rc.Name, expectedName)
			if rc.Name == expectedName {
				found = true
				if strings.HasPrefix(rc.State, "Exit") {
					log.Printf("Container %s exited. Restarting...", specContainer.Name)
					a.Runtime.StopContainer(a.ctx, rc.ID, 0)
					a.Runtime.RunContainer(a.ctx, pod, &specContainer)
				} else {
					// Pending/Running. Get IP.
					if ip, err := a.Runtime.GetContainerIP(a.ctx, rc.ID); err == nil && ip != "" {
						pod.Status.PodIP = ip
					}

					// Running. Check Probes.
					if specContainer.LivenessProbe != nil {
						ok, err := a.Prober.Probe(pod, &specContainer, specContainer.LivenessProbe)
						if err != nil {
							log.Printf("Probe error for %s: %v", specContainer.Name, err)
						} else if !ok {
							log.Printf("Liveness probe failed for %s. Restarting...", specContainer.Name)
							a.Runtime.StopContainer(a.ctx, rc.ID, 1) // Graceful stop
							a.Runtime.RunContainer(a.ctx, pod, &specContainer)
						}
					}
				}
				break
			}
		}

		if !found {
			log.Printf("Starting container %s for pod %s", specContainer.Name, pod.Name)
			if _, err := a.Runtime.RunContainer(a.ctx, pod, &specContainer); err != nil {
				log.Printf("Error running container: %v", err)
			}
		}
	}

	// Update Status to API
	// Naive: If at least one container running -> Running.
	// If all containers done -> Succeeded?
	// Let's just set to Running if we found containers.
	newPhase := "Pending"
	if len(runningContainers) > 0 {
		newPhase = "Running"
	}

	// Only update if changed (simple cache/check logic?)
	// For MVP, just update periodically or check if pod.Status.Phase != newPhase
	// Also check if PodIP changed
	if pod.Status.Phase != newPhase || pod.Status.PodIP != "" {
		pod.Status.Phase = newPhase
		if err := a.Client.UpdatePodStatus(a.ctx, pod); err != nil {
			log.Printf("Failed to update pod status: %v", err)
		} else {
			log.Printf("Updated Pod %s status to %s", pod.Name, newPhase)
		}
	}
}



