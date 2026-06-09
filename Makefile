APP := kang-agent
IMAGE ?= ghcr.io/project-kang/kang-agent:dev

.PHONY: build
build:
	go build -o bin/$(APP) ./cmd/kang-agent

.PHONY: test
test:
	go test ./...

.PHONY: run-agent
run-agent:
	go run ./cmd/kang-agent --listen=:8081 --host-id=local-dev

.PHONY: image
image:
	docker build -t $(IMAGE) .

.PHONY: k8s-apply
k8s-apply:
	kubectl apply -f deployments/kubernetes/kang-agent-daemonset.yaml
