package kubelet

import (
	"context"
	"fmt"
	"os/exec"
	"strings"

	"github.com/abhigod/k8s-lite/internal/api"
)

// Runtime abstracts the container engine (Docker, etc)
type Runtime interface {
	RunContainer(ctx context.Context, pod *api.Pod, container *api.Container) (string, error)
	StopContainer(ctx context.Context, containerID string, timeoutSeconds int) error
	ListContainers(ctx context.Context) ([]ContainerInfo, error)
	GetContainerIP(ctx context.Context, containerID string) (string, error)
}

type ContainerInfo struct {
	ID           string
	Name         string
	Image        string
	State        string // running, exited
	PodName      string // stored in label
	PodNamespace string
}

// DockerRuntime implements Runtime using the 'docker' CLI.
type DockerRuntime struct{}

func NewDockerRuntime() *DockerRuntime {
	return &DockerRuntime{}
}

func (d *DockerRuntime) RunContainer(ctx context.Context, pod *api.Pod, container *api.Container) (string, error) {
	// k8s-lite-podName-containerName
	containerName := fmt.Sprintf("k8s-lite-%s-%s", pod.Name, container.Name)

	// Check if running
	// For simplicity, always remove and recreate if not restart=Never?
	// Proper Kubelet checks hash. We will just check existence.

	// docker run -d --name <name> --label k8s-app=<podName> <image> <args>

	args := []string{"run", "-d", "--name", containerName}
	args = append(args, "--label", fmt.Sprintf("k8s.pod.name=%s", pod.Name))
	args = append(args, "--label", fmt.Sprintf("k8s.pod.namespace=%s", pod.Namespace))
	// args = append(args, "--network", "host") // Removed to allow bridge networking (unique IPs per pod)

	// Ports -p host:container (Skipped due to host network)

	for range container.Ports {
		// Simple host port mapping match if specified, else random?
		// K8s usually assigns PodIP. Accessing via host port needs hostPort.
		// For MVP, lets just publish all exposed ports 1:1 if possible or random.
		// Or if HostPort is not set, we don't expose?
		// Let's use -P (publish all) or just -p 80:80 if easy? NO, port collision.
		// Let's just pass containerPort for now, but without host mapping unless we use CNI.
		// Docker default network gives IP.
	}

	args = append(args, container.Image)
	args = append(args, container.Command...)

	cmd := exec.CommandContext(ctx, "docker", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("docker run failed: %s, output: %s", err, string(out))
	}

	id := strings.TrimSpace(string(out))
	return id, nil
}

func (d *DockerRuntime) StopContainer(ctx context.Context, containerID string, timeoutSeconds int) error {
	// docker stop -t <seconds> <id>
	args := []string{"stop"}
	if timeoutSeconds > 0 {
		args = append(args, "-t", fmt.Sprintf("%d", timeoutSeconds))
	}
	args = append(args, containerID)

	cmd := exec.CommandContext(ctx, "docker", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("docker stop failed: %s, output: %s", err, string(out))
	}

	// Auto-remove after stop (since we run with -d, it stays).
	// Kubelet usually keeps dead containers for log viewing until GC.
	// We will just remove it for MVP simplicity implementation of "Restart" usually deletions old one.
	// If we want logs, we shouldn't remove immediately.
	// But our agent.go restarts by Stop+Run.
	// Let's remove it explicitly here to clean up.
	exec.CommandContext(ctx, "docker", "rm", containerID).Run()

	return nil
}

func (d *DockerRuntime) ListContainers(ctx context.Context) ([]ContainerInfo, error) {
	// docker ps -a --format "{{.ID}}|{{.Names}}|{{.Image}}|{{.Status}}|{{.Label \"k8s.pod.name\"}}"
	cmd := exec.CommandContext(ctx, "docker", "ps", "-a", "--format", "{{.ID}}|{{.Names}}|{{.Image}}|{{.State}}|{{.Label \"k8s.pod.name\"}}|{{.Label \"k8s.pod.namespace\"}}")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, err
	}

	var containers []ContainerInfo
	lines := strings.Split(string(out), "\n")
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		parts := strings.Split(line, "|")
		if len(parts) < 6 {
			continue
		}

		// Filter only ours
		if parts[4] == "" { // No pod label
			continue
		}

		containers = append(containers, ContainerInfo{
			ID:           parts[0],
			Name:         parts[1],
			Image:        parts[2],
			State:        parts[3], // running, exited
			PodName:      parts[4],
			PodNamespace: parts[5],
		})
	}
	return containers, nil
}

func (d *DockerRuntime) GetContainerIP(ctx context.Context, containerID string) (string, error) {
	// docker inspect -f '{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}' <id>
	// Note: Use range because network name might vary (bridge, custom).
	cmd := exec.CommandContext(ctx, "docker", "inspect", "-f", "{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}", containerID)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("docker inspect ip failed: %s, output: %s", err, string(out))
	}
	return strings.TrimSpace(string(out)), nil
}





