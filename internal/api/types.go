package api

import (
	"encoding/json"
	"fmt"
	"time"
)

// TypeMeta describes an individual object in an API response or request
type TypeMeta struct {
	Kind       string `json:"kind,omitempty"`
	APIVersion string `json:"apiVersion,omitempty"`
}

// ObjectMeta is metadata that all persisted resources must have
type ObjectMeta struct {
	Name              string            `json:"name,omitempty"`
	Namespace         string            `json:"namespace,omitempty"` // default is "default"
	Labels            map[string]string `json:"labels,omitempty"`
	Annotations       map[string]string `json:"annotations,omitempty"`
	ResourceVersion   string            `json:"resourceVersion,omitempty"`
	CreationTimestamp time.Time         `json:"creationTimestamp,omitempty"`
	DeletionTimestamp *time.Time        `json:"deletionTimestamp,omitempty"`
}

// Pod is a collection of containers that can run on a host.
type Pod struct {
	TypeMeta   `json:",inline"`
	ObjectMeta `json:"metadata,omitempty"`

	Spec   PodSpec   `json:"spec,omitempty"`
	Status PodStatus `json:"status,omitempty"`
}

type PodSpec struct {
	Containers    []Container `json:"containers"`
	NodeName      string      `json:"nodeName,omitempty"`      // schedulable
	RestartPolicy string      `json:"restartPolicy,omitempty"` // Always, OnFailure, Never
}

type Container struct {
	Name           string               `json:"name"`
	Image          string               `json:"image"`
	Command        []string             `json:"command,omitempty"`
	Args           []string             `json:"args,omitempty"`
	Ports          []ContainerPort      `json:"ports,omitempty"`
	Resources      ResourceRequirements `json:"resources,omitempty"`
	LivenessProbe  *Probe               `json:"livenessProbe,omitempty"`
	ReadinessProbe *Probe               `json:"readinessProbe,omitempty"`
}

type ContainerPort struct {
	Name          string `json:"name,omitempty"`
	ContainerPort int    `json:"containerPort"`
	Protocol      string `json:"protocol,omitempty"` // TCP/UDP
}

type ResourceRequirements struct {
	Requests ResourceList `json:"requests,omitempty"`
	Limits   ResourceList `json:"limits,omitempty"`
}

type Probe struct {
	HTTPGet             *HTTPGetAction   `json:"httpGet,omitempty"`
	TCPSocket           *TCPSocketAction `json:"tcpSocket,omitempty"`
	InitialDelaySeconds int32            `json:"initialDelaySeconds,omitempty"`
	PeriodSeconds       int32            `json:"periodSeconds,omitempty"`
}

type HTTPGetAction struct {
	Path string      `json:"path,omitempty"`
	Port IntOrString `json:"port"`
}

type TCPSocketAction struct {
	Port IntOrString `json:"port"`
}

type IntOrString struct {
	Type   int    `json:"type"` // 0: Int, 1: String
	IntVal int32  `json:"intVal"`
	StrVal string `json:"strVal"`
}

func (i IntOrString) String() string {
	if i.Type == 0 {
		return fmt.Sprintf("%d", i.IntVal)
	}
	return i.StrVal
}

func (i IntOrString) MarshalJSON() ([]byte, error) {
	if i.Type == 0 {
		return json.Marshal(i.IntVal)
	}
	return json.Marshal(i.StrVal)
}

func (i *IntOrString) UnmarshalJSON(data []byte) error {
	if len(data) == 0 {
		return nil
	}
	if data[0] == '"' {
		var s string
		if err := json.Unmarshal(data, &s); err != nil {
			return err
		}
		i.Type = 1
		i.StrVal = s
		return nil
	}
	var n int32
	if err := json.Unmarshal(data, &n); err != nil {
		return err
	}
	i.Type = 0
	i.IntVal = n
	return nil
}

type ResourceList map[string]string // e.g. "cpu": "100m", "memory": "128Mi"

type PodStatus struct {
	Phase             string            `json:"phase,omitempty"` // Pending, Running, Succeeded, Failed, Unknown
	Conditions        []PodCondition    `json:"conditions,omitempty"`
	HostIP            string            `json:"hostIP,omitempty"`
	PodIP             string            `json:"podIP,omitempty"`
	ContainerStatuses []ContainerStatus `json:"containerStatuses,omitempty"`
}

type PodCondition struct {
	Type   string `json:"type"`   // Ready, Scheduled
	Status string `json:"status"` // True, False, Unknown
}

type ContainerStatus struct {
	Name         string         `json:"name"`
	State        ContainerState `json:"state"`
	Ready        bool           `json:"ready"`
	RestartCount int            `json:"restartCount"`
	Image        string         `json:"image"`
	ContainerID  string         `json:"containerID,omitempty"`
}

type ContainerState struct {
	Running    *ContainerStateRunning    `json:"running,omitempty"`
	Terminated *ContainerStateTerminated `json:"terminated,omitempty"`
	Waiting    *ContainerStateWaiting    `json:"waiting,omitempty"`
}

type ContainerStateRunning struct {
	StartedAt time.Time `json:"startedAt,omitempty"`
}

type ContainerStateTerminated struct {
	ExitCode   int       `json:"exitCode"`
	Reason     string    `json:"reason,omitempty"`
	Message    string    `json:"message,omitempty"`
	StartedAt  time.Time `json:"startedAt,omitempty"`
	FinishedAt time.Time `json:"finishedAt,omitempty"`
}

type ContainerStateWaiting struct {
	Reason  string `json:"reason,omitempty"`
	Message string `json:"message,omitempty"`
}

// Node is a worker node in the cluster
type Node struct {
	TypeMeta   `json:",inline"`
	ObjectMeta `json:"metadata,omitempty"`

	Spec   NodeSpec   `json:"spec,omitempty"`
	Status NodeStatus `json:"status,omitempty"`
}

type NodeSpec struct {
	Unschedulable bool   `json:"unschedulable,omitempty"`
	PodCIDR       string `json:"podCIDR,omitempty"`
}

type NodeStatus struct {
	Capacity    ResourceList    `json:"capacity,omitempty"`
	Allocatable ResourceList    `json:"allocatable,omitempty"`
	Conditions  []NodeCondition `json:"conditions,omitempty"`
	Addresses   []NodeAddress   `json:"addresses,omitempty"`
	NodeInfo    NodeSystemInfo  `json:"nodeInfo,omitempty"`
}

type NodeCondition struct {
	Type              string    `json:"type"`   // Ready, MemoryPressure, DiskPressure
	Status            string    `json:"status"` // True, False, Unknown
	LastHeartbeatTime time.Time `json:"lastHeartbeatTime,omitempty"`
}

type NodeAddress struct {
	Type    string `json:"type"` // Hostname, InternalIP
	Address string `json:"address"`
}

type NodeSystemInfo struct {
	MachineID               string `json:"machineID"`
	SystemUUID              string `json:"systemUUID"`
	BootID                  string `json:"bootID"`
	KernelVersion           string `json:"kernelVersion"`
	OSImage                 string `json:"osImage"`
	ContainerRuntimeVersion string `json:"containerRuntimeVersion"`
	KubeletVersion          string `json:"kubeletVersion"`
	OperatingSystem         string `json:"operatingSystem"`
	Architecture            string `json:"architecture"`
}

// ReplicaSet ensures that a specified number of pod replicas are running at any given time.
type ReplicaSet struct {
	TypeMeta   `json:",inline"`
	ObjectMeta `json:"metadata,omitempty"`

	Spec   ReplicaSetSpec   `json:"spec,omitempty"`
	Status ReplicaSetStatus `json:"status,omitempty"`
}

type ReplicaSetSpec struct {
	Replicas *int32          `json:"replicas,omitempty"`
	Selector LabelSelector   `json:"selector"`
	Template PodTemplateSpec `json:"template,omitempty"`
}

type ReplicaSetStatus struct {
	Replicas             int32 `json:"replicas"`
	FullyLabeledReplicas int32 `json:"fullyLabeledReplicas,omitempty"`
	ReadyReplicas        int32 `json:"readyReplicas,omitempty"`
	AvailableReplicas    int32 `json:"availableReplicas,omitempty"`
}

type LabelSelector struct {
	MatchLabels map[string]string `json:"matchLabels,omitempty"`
}

type PodTemplateSpec struct {
	ObjectMeta `json:"metadata,omitempty"`
	Spec       PodSpec `json:"spec,omitempty"`
}

// List types
type PodList struct {
	TypeMeta `json:",inline"`
	ListMeta `json:"metadata,omitempty"`
	Items    []Pod `json:"items"`
}

type NodeList struct {
	TypeMeta `json:",inline"`
	ListMeta `json:"metadata,omitempty"`
	Items    []Node `json:"items"`
}

type ReplicaSetList struct {
	TypeMeta `json:",inline"`
	ListMeta `json:"metadata,omitempty"`
	Items    []ReplicaSet `json:"items"`
}

type ListMeta struct {
	ResourceVersion string `json:"resourceVersion,omitempty"`
}

// Deployment enables declarative updates for Pods and ReplicaSets.
type Deployment struct {
	TypeMeta   `json:",inline"`
	ObjectMeta `json:"metadata,omitempty"`

	Spec   DeploymentSpec   `json:"spec,omitempty"`
	Status DeploymentStatus `json:"status,omitempty"`
}

type DeploymentSpec struct {
	Replicas *int32             `json:"replicas,omitempty"`
	Selector LabelSelector      `json:"selector"`
	Template PodTemplateSpec    `json:"template"`
	Strategy DeploymentStrategy `json:"strategy,omitempty"`
}

type DeploymentStrategy struct {
	Type          string                   `json:"type,omitempty"` // RollingUpdate, Recreate
	RollingUpdate *RollingUpdateDeployment `json:"rollingUpdate,omitempty"`
}

type RollingUpdateDeployment struct {
	MaxUnavailable *IntOrString `json:"maxUnavailable,omitempty"`
	MaxSurge       *IntOrString `json:"maxSurge,omitempty"`
}

type DeploymentStatus struct {
	ObservedGeneration  int64 `json:"observedGeneration,omitempty"`
	Replicas            int32 `json:"replicas,omitempty"`
	UpdatedReplicas     int32 `json:"updatedReplicas,omitempty"`
	ReadyReplicas       int32 `json:"readyReplicas,omitempty"`
	AvailableReplicas   int32 `json:"availableReplicas,omitempty"`
	UnavailableReplicas int32 `json:"unavailableReplicas,omitempty"`
}

type DeploymentList struct {
	TypeMeta `json:",inline"`
	Items    []Deployment `json:"items"`
}

// Service provides a stable endpoint for a set of Pods.
type Service struct {
	TypeMeta   `json:",inline"`
	ObjectMeta `json:"metadata,omitempty"`

	Spec   ServiceSpec   `json:"spec,omitempty"`
	Status ServiceStatus `json:"status,omitempty"`
}

type ServiceSpec struct {
	Selector  map[string]string `json:"selector,omitempty"`
	Ports     []ServicePort     `json:"ports,omitempty"`
	Type      string            `json:"type,omitempty"` // ClusterIP, NodePort
	ClusterIP string            `json:"clusterIP,omitempty"`
}

type ServicePort struct {
	Name       string      `json:"name,omitempty"`
	Protocol   string      `json:"protocol,omitempty"` // TCP/UDP
	Port       int32       `json:"port"`
	TargetPort IntOrString `json:"targetPort,omitempty"` // Port on the pod
	NodePort   int32       `json:"nodePort,omitempty"`
}

type ServiceStatus struct {
	LoadBalancer LoadBalancerStatus `json:"loadBalancer,omitempty"`
}

type LoadBalancerStatus struct {
	Ingress []LoadBalancerIngress `json:"ingress,omitempty"`
}

type LoadBalancerIngress struct {
	IP       string `json:"ip,omitempty"`
	Hostname string `json:"hostname,omitempty"`
}

type ServiceList struct {
	TypeMeta `json:",inline"`
	ListMeta `json:"metadata,omitempty"`
	Items    []Service `json:"items"`
}

// Endpoints groups the ready IPs of Pods backing a Service.
type Endpoints struct {
	TypeMeta   `json:",inline"`
	ObjectMeta `json:"metadata,omitempty"`

	Subsets []EndpointSubset `json:"subsets,omitempty"`
}

type EndpointSubset struct {
	Addresses []EndpointAddress `json:"addresses,omitempty"`
	Ports     []EndpointPort    `json:"ports,omitempty"`
}

type EndpointAddress struct {
	IP       string `json:"ip"`
	NodeName string `json:"nodeName,omitempty"`
}

type EndpointPort struct {
	Name     string `json:"name,omitempty"`
	Port     int32  `json:"port"`
	Protocol string `json:"protocol,omitempty"`
}

type EndpointsList struct {
	TypeMeta `json:",inline"`
	ListMeta `json:"metadata,omitempty"`
	Items    []Endpoints `json:"items"`
}

// Lease defines a lease concept for leader election.
type Lease struct {
	TypeMeta   `json:",inline"`
	ObjectMeta `json:"metadata,omitempty"`

	Spec LeaseSpec `json:"spec,omitempty"`
}

type LeaseSpec struct {
	HolderIdentity       *string    `json:"holderIdentity,omitempty"`
	LeaseDurationSeconds *int32     `json:"leaseDurationSeconds,omitempty"`
	AcquireTime          *time.Time `json:"acquireTime,omitempty"`
	RenewTime            *time.Time `json:"renewTime,omitempty"`
	LeaseTransitions     *int32     `json:"leaseTransitions,omitempty"`
}

type LeaseList struct {
	TypeMeta `json:",inline"`
	ListMeta `json:"metadata,omitempty"`
	Items    []Lease `json:"items"`
}




