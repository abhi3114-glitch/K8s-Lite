package kubelet

import (
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/abhigod/k8s-lite/internal/api"
)

type Prober interface {
	Probe(pod *api.Pod, container *api.Container, probe *api.Probe) (bool, error)
}

type DefaultProber struct {
	HTTP *http.Client
}

func NewProber() *DefaultProber {
	return &DefaultProber{
		HTTP: &http.Client{
			Timeout: 2 * time.Second,
		},
	}
}

func (p *DefaultProber) Probe(pod *api.Pod, container *api.Container, probe *api.Probe) (bool, error) {
	if probe.HTTPGet != nil {
		return p.probeHTTP(pod, container, probe.HTTPGet)
	}
	if probe.TCPSocket != nil {
		return p.probeTCP(pod, container, probe.TCPSocket)
	}
	return true, nil // No probe defined implies success
}

func (p *DefaultProber) probeHTTP(pod *api.Pod, container *api.Container, w *api.HTTPGetAction) (bool, error) {
	host := pod.Status.PodIP
	if host == "" {
		host = "localhost"
	}
	port := w.Port.String() // IntOrString
	url := fmt.Sprintf("http://%s:%s%s", host, port, w.Path)

	resp, err := p.HTTP.Get(url)
	if err != nil {
		return false, nil // Connection refused = fail
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 400 {
		return true, nil
	}
	return false, nil
}

func (p *DefaultProber) probeTCP(pod *api.Pod, container *api.Container, t *api.TCPSocketAction) (bool, error) {
	host := pod.Status.PodIP
	if host == "" {
		host = "localhost"
	}
	port := t.Port.String()
	conn, err := net.DialTimeout("tcp", net.JoinHostPort(host, port), 2*time.Second)
	if err != nil {
		return false, nil
	}
	conn.Close()
	return true, nil
}





