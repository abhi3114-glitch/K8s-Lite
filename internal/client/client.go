package client

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"

	"github.com/abhigod/k8s-lite/internal/api"
)

type Client struct {
	BaseURL string
	HTTP    *http.Client
}

func New(baseURL string, tlsCert, tlsKey, tlsCA string) *Client {
	httpClient := &http.Client{}

	if tlsCert != "" && tlsKey != "" && tlsCA != "" {
		// Load Client Cert
		cert, err := tls.LoadX509KeyPair(tlsCert, tlsKey)
		if err != nil {
			log.Fatalf("Failed to load client cert: %v", err)
		}

		// Load CA
		caCert, err := ioutil.ReadFile(tlsCA)
		if err != nil {
			log.Fatalf("Failed to read CA cert: %v", err)
		}
		caCertPool := x509.NewCertPool()
		caCertPool.AppendCertsFromPEM(caCert)

		httpClient.Transport = &http.Transport{
			TLSClientConfig: &tls.Config{
				Certificates: []tls.Certificate{cert},
				RootCAs:      caCertPool,
			},
		}
	}

	return &Client{
		BaseURL: baseURL,
		HTTP:    httpClient,
	}
}

// Pods

func (c *Client) ListPods(ctx context.Context, nodeName string) ([]api.Pod, error) {
	url := fmt.Sprintf("%s/api/v1/pods", c.BaseURL)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("api request failed: %s", resp.Status)
	}

	var list api.PodList
	if err := json.NewDecoder(resp.Body).Decode(&list); err != nil {
		return nil, err
	}

	return list.Items, nil
}

func (c *Client) UpdatePod(ctx context.Context, pod *api.Pod) error {
	url := fmt.Sprintf("%s/api/v1/pods/%s", c.BaseURL, pod.Name)
	data, err := json.Marshal(pod)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, "PUT", url, bytes.NewBuffer(data))
	if err != nil {
		return err
	}

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to update pod: %s", resp.Status)
	}
	return nil
}

func (c *Client) UpdatePodStatus(ctx context.Context, pod *api.Pod) error {
	return c.UpdatePod(ctx, pod)
}

func (c *Client) CreatePod(ctx context.Context, pod *api.Pod) error {
	url := fmt.Sprintf("%s/api/v1/pods", c.BaseURL)
	data, err := json.Marshal(pod)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(data))
	if err != nil {
		return err
	}

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to create pod: %s", resp.Status)
	}
	return nil
}

func (c *Client) DeletePod(ctx context.Context, name string) error {
	url := fmt.Sprintf("%s/api/v1/pods/%s", c.BaseURL, name)
	req, err := http.NewRequestWithContext(ctx, "DELETE", url, nil)
	if err != nil {
		return err
	}

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		if resp.StatusCode == http.StatusNotFound {
			return nil
		}
		return fmt.Errorf("failed to delete pod: %s", resp.Status)
	}
	return nil
}

// Nodes

func (c *Client) RegisterNode(ctx context.Context, node *api.Node) error {
	url := fmt.Sprintf("%s/api/v1/nodes", c.BaseURL)

	data, err := json.Marshal(node)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(data))
	if err != nil {
		return err
	}

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusConflict {
		return nil
	}

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to register node: %s", resp.Status)
	}
	return nil
}

func (c *Client) ListNodes(ctx context.Context) ([]api.Node, error) {
	url := fmt.Sprintf("%s/api/v1/nodes", c.BaseURL)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("api request failed: %s", resp.Status)
	}

	var list api.NodeList
	if err := json.NewDecoder(resp.Body).Decode(&list); err != nil {
		return nil, err
	}

	return list.Items, nil
}

// ReplicaSets

func (c *Client) ListReplicaSets(ctx context.Context) ([]api.ReplicaSet, error) {
	url := fmt.Sprintf("%s/apis/apps/v1/replicasets", c.BaseURL)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("api request failed: %s", resp.Status)
	}

	var list api.ReplicaSetList
	if err := json.NewDecoder(resp.Body).Decode(&list); err != nil {
		return nil, err
	}

	return list.Items, nil
}

func (c *Client) CreateReplicaSet(ctx context.Context, rs *api.ReplicaSet) error {
	url := fmt.Sprintf("%s/apis/apps/v1/replicasets", c.BaseURL)
	data, err := json.Marshal(rs)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(data))
	if err != nil {
		return err
	}

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to create replicaset: %s", resp.Status)
	}
	return nil
}

func (c *Client) UpdateReplicaSet(ctx context.Context, rs *api.ReplicaSet) error {
	url := fmt.Sprintf("%s/apis/apps/v1/replicasets/%s", c.BaseURL, rs.Name)
	data, err := json.Marshal(rs)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, "PUT", url, bytes.NewBuffer(data))
	if err != nil {
		return err
	}

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to update replicaset: %s", resp.Status)
	}
	return nil
}

// Deployments

func (c *Client) ListDeployments(ctx context.Context) ([]api.Deployment, error) {
	url := fmt.Sprintf("%s/apis/apps/v1/deployments", c.BaseURL)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("api request failed: %s", resp.Status)
	}

	var list api.DeploymentList
	if err := json.NewDecoder(resp.Body).Decode(&list); err != nil {
		return nil, err
	}

	return list.Items, nil
}

func (c *Client) UpdateDeployment(ctx context.Context, deploy *api.Deployment) error {
	url := fmt.Sprintf("%s/apis/apps/v1/deployments/%s", c.BaseURL, deploy.Name)
	data, err := json.Marshal(deploy)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, "PUT", url, bytes.NewBuffer(data))
	if err != nil {
		return err
	}

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to update deployment: %s", resp.Status)
	}
	return nil
}

func (c *Client) CreateDeployment(ctx context.Context, deploy *api.Deployment) error {
	url := fmt.Sprintf("%s/apis/apps/v1/deployments", c.BaseURL)
	data, err := json.Marshal(deploy)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(data))
	if err != nil {
		return err
	}

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to create deployment: %s", resp.Status)
	}
	return nil
}

// Services

func (c *Client) ListServices(ctx context.Context) ([]api.Service, error) {
	url := fmt.Sprintf("%s/api/v1/services", c.BaseURL)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("api request failed: %s", resp.Status)
	}

	var list api.ServiceList
	if err := json.NewDecoder(resp.Body).Decode(&list); err != nil {
		return nil, err
	}

	return list.Items, nil
}

func (c *Client) CreateService(ctx context.Context, svc *api.Service) error {
	url := fmt.Sprintf("%s/api/v1/services", c.BaseURL)
	data, err := json.Marshal(svc)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(data))
	if err != nil {
		return err
	}

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to create service: %s", resp.Status)
	}
	return nil
}

// Endpoints

func (c *Client) GetEndpoints(ctx context.Context, name string) (*api.Endpoints, error) {
	url := fmt.Sprintf("%s/api/v1/endpoints/%s", c.BaseURL, name)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		if resp.StatusCode == http.StatusNotFound {
			return nil, nil // Not found
		}
		return nil, fmt.Errorf("api request failed: %s", resp.Status)
	}

	var ep api.Endpoints
	if err := json.NewDecoder(resp.Body).Decode(&ep); err != nil {
		return nil, err
	}

	return &ep, nil
}

func (c *Client) CreateEndpoints(ctx context.Context, ep *api.Endpoints) error {
	url := fmt.Sprintf("%s/api/v1/endpoints", c.BaseURL)
	data, err := json.Marshal(ep)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(data))
	if err != nil {
		return err
	}

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to create endpoints: %s", resp.Status)
	}
	return nil
}

func (c *Client) UpdateEndpoints(ctx context.Context, ep *api.Endpoints) error {
	url := fmt.Sprintf("%s/api/v1/endpoints/%s", c.BaseURL, ep.Name)
	data, err := json.Marshal(ep)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, "PUT", url, bytes.NewBuffer(data))
	if err != nil {
		return err
	}

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to update endpoints: %s", resp.Status)
	}
	return nil
}

// Leases

func (c *Client) GetLease(ctx context.Context, name string) (*api.Lease, error) {
	url := fmt.Sprintf("%s/api/v1/leases/%s", c.BaseURL, name)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		if resp.StatusCode == http.StatusNotFound {
			return nil, nil // Not found
		}
		return nil, fmt.Errorf("api request failed: %s", resp.Status)
	}

	var lease api.Lease
	if err := json.NewDecoder(resp.Body).Decode(&lease); err != nil {
		return nil, err
	}

	return &lease, nil
}

func (c *Client) CreateLease(ctx context.Context, lease *api.Lease) error {
	url := fmt.Sprintf("%s/api/v1/leases", c.BaseURL)
	data, err := json.Marshal(lease)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(data))
	if err != nil {
		return err
	}

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to create lease: %s", resp.Status)
	}
	return nil
}

func (c *Client) UpdateLease(ctx context.Context, lease *api.Lease) error {
	url := fmt.Sprintf("%s/api/v1/leases/%s", c.BaseURL, lease.Name)
	data, err := json.Marshal(lease)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, "PUT", url, bytes.NewBuffer(data))
	if err != nil {
		return err
	}

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to update lease: %s", resp.Status)
	}
	return nil
}








