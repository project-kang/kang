# Kang

Kang is a managed Kubernetes platform for bare metal environments.

The long-term goal of Kang is:

```text
GKE for bare metal
```

Kang is being built for cloud providers, MSPs, enterprises, and bare metal operators who want to offer or operate managed Kubernetes clusters without depending on a full traditional IaaS stack.

## Vision

Kang aims to provide managed Kubernetes on bare metal without requiring:

```text
OpenStack
KubeVirt
Cluster API
an existing Kubernetes-based infrastructure platform
```

Cluster API support may exist in the future, but Kang itself is the product.

## Why Kang

Current approaches have tradeoffs.

OpenStack is mature and widely adopted, but it is operationally heavy and solves much more than managed Kubernetes.

KubeVirt is Kubernetes-native, but it requires Kubernetes to manage Kubernetes and adds extra networking and lifecycle complexity.

Custom VM automation is lightweight, but every operator ends up rebuilding similar infrastructure again and again.

Kang focuses on a smaller goal:

```text
Bare Metal
    ↓
Kang
    ↓
VM-backed Kubernetes Clusters
```

## Current Status

Kang is in early development.

The current focus is Phase 0:

```text
VM lifecycle on bare metal hosts
```

The first functional milestone is `kang-agent`, a host-level agent that can expose host inventory and manage local VMs using libvirt.

## Current Functionality

The current implementation supports:

```text
agent process
health endpoint
host inventory endpoint
VM create API
VM list API
VM delete API
qcow2 disk creation
libvirt domain creation
libvirt VM start
libvirt VM destroy
VM disk cleanup
```

Current runtime flow:

```text
API client
    ↓
kang-agent
    ↓
VM driver
    ↓
qemu-img
    ↓
virsh / libvirt
    ↓
real VM on bare metal
```

## Components

### kang-agent

`kang-agent` runs on a bare metal host.

It is responsible for:

```text
reporting host inventory
creating VM disks
creating libvirt domains
starting VMs
listing Kang-managed VMs
deleting VMs
```

Current supported VM backend:

```text
virsh / libvirt
```

Future backends may include:

```text
OpenStack
KubeVirt
Firecracker
cloud-hypervisor
custom virtualization backends
```

## API

### Health

```text
GET /healthz
```

Example:

```bash
curl localhost:8081/healthz
```

Response:

```text
ok
```

### Host Inventory

```text
GET /v1/host
```

Example:

```bash
curl localhost:8081/v1/host | jq
```

Example response:

```json
{
  "id": "bm-1",
  "cpu": 32,
  "memory_mb": 257000,
  "hostname": "worker-1",
  "os": "linux",
  "arch": "amd64",
  "driver": "virsh"
}
```

### Create VM

```text
POST /v1/vms
```

Example:

```bash
curl -X POST localhost:8081/v1/vms \
  -H "Content-Type: application/json" \
  -d '{
    "name": "test-vm-1",
    "cpu": 2,
    "memory_mb": 2048,
    "base_image_path": "/var/lib/libvirt/images/ubuntu-22.04-base.qcow2",
    "disk_gb": 20
  }' | jq
```

Kang creates a per-VM qcow2 disk using the provided base image.

Example generated disk:

```text
/var/lib/libvirt/images/kang-<vm-id>.qcow2
```

Example response:

```json
{
  "vm": {
    "id": "2f8ce8c0",
    "name": "test-vm-1",
    "domain_name": "kang-2f8ce8c0",
    "cpu": 2,
    "memory_mb": 2048,
    "disk_gb": 20,
    "base_image_path": "/var/lib/libvirt/images/ubuntu-22.04-base.qcow2",
    "disk_path": "/var/lib/libvirt/images/kang-2f8ce8c0.qcow2",
    "state": "running",
    "created_at": "2026-06-09T12:37:40Z"
  }
}
```

### List VMs

```text
GET /v1/vms
```

Example:

```bash
curl localhost:8081/v1/vms | jq
```

### Delete VM

```text
DELETE /v1/vms/{id}
```

Example:

```bash
curl -X DELETE localhost:8081/v1/vms/2f8ce8c0 | jq
```

Delete currently performs:

```text
virsh destroy
virsh undefine
qcow2 disk removal
```

## Running Locally

Install dependencies:

```bash
sudo apt-get update
sudo apt-get install -y qemu-kvm libvirt-daemon-system libvirt-clients qemu-utils
sudo systemctl enable --now libvirtd
```

Ensure the default libvirt network exists:

```bash
sudo virsh net-list --all
sudo virsh net-start default || true
sudo virsh net-autostart default || true
```

Prepare a base image:

```bash
sudo mkdir -p /var/lib/libvirt/images

sudo wget -O /var/lib/libvirt/images/ubuntu-22.04-base.qcow2 \
  https://cloud-images.ubuntu.com/jammy/current/jammy-server-cloudimg-amd64.img
```

Run the agent:

```bash
go run ./cmd/kang-agent \
  --listen=:8081 \
  --host-id=bm-1 \
  --vm-driver=virsh \
  --virsh-path=virsh \
  --qemu-img-path=qemu-img \
  --libvirt-network=default \
  --image-dir=/var/lib/libvirt/images
```

## Build

```bash
go build -o bin/kang-agent ./cmd/kang-agent
```

## Repository Structure

```text
cmd/
└── kang-agent/
    └── main.go

internal/
├── agent/
│   ├── host.go
│   └── server.go
└── vm/
    ├── driver.go
    └── virsh.go

pkg/
└── types/
    └── types.go
```

## Design Principles

Kang is designed around these principles:

```text
bare-metal first
managed Kubernetes focused
lightweight infrastructure layer
simple host agent model
pluggable VM backends
container-compatible
Kubernetes-deployable when needed
not dependent on Kubernetes
not dependent on OpenStack
not dependent on KubeVirt
```

## Roadmap

Near-term:

```text
cloud-init support
SSH key injection
VM network discovery
VM status refresh
VM stop/start/reboot APIs
kangctl CLI
kang-api skeleton
host registration
agent heartbeat
```

Future:

```text
multi-host scheduling
managed Kubernetes cluster creation
node pool management
OpenTelemetry integration
OpenStack backend
Kubernetes deployment model
Cluster API provider
extension model
multi-tenancy
authentication and authorization
```

## License

Apache-2.0

