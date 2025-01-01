.PHONY: build test clean

BINARY_DIR=bin

build: build-apiserver build-scheduler build-controller-manager build-kubelet build-kubectl

build-apiserver:
	go build -o $(BINARY_DIR)/apiserver ./cmd/apiserver

build-scheduler:
	go build -o $(BINARY_DIR)/scheduler ./cmd/scheduler

build-controller-manager:
	go build -o $(BINARY_DIR)/controller-manager ./cmd/controller-manager

build-kubelet:
	go build -o $(BINARY_DIR)/kubelet ./cmd/kubelet

build-kubectl:
	go build -o $(BINARY_DIR)/kubectl-lite ./cmd/kubectl-lite

test:
	go test ./...

clean:
	rm -rf $(BINARY_DIR)
