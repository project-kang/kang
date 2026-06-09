package types

import "time"

type HostInfo struct {
	ID       string `json:"id"`
	CPU      int    `json:"cpu"`
	MemoryMB int    `json:"memory_mb"`
	Hostname string `json:"hostname"`
	OS       string `json:"os"`
	Arch     string `json:"arch"`
	Driver   string `json:"driver"`
}

type VMState string

const (
	VMStateRunning VMState = "running"
	VMStateStopped VMState = "stopped"
	VMStateUnknown VMState = "unknown"
)

type VM struct {
	ID            string    `json:"id"`
	Name          string    `json:"name"`
	DomainName    string    `json:"domain_name"`
	CPU           int       `json:"cpu"`
	MemoryMB      int       `json:"memory_mb"`
	DiskGB        int       `json:"disk_gb"`
	BaseImagePath string    `json:"base_image_path,omitempty"`
	DiskPath      string    `json:"disk_path,omitempty"`
	State         VMState   `json:"state"`
	CreatedAt     time.Time `json:"created_at"`
}

type CreateVMRequest struct {
	Name          string `json:"name"`
	CPU           int    `json:"cpu"`
	MemoryMB      int    `json:"memory_mb"`
	BaseImagePath string `json:"base_image_path"`
	DiskGB        int    `json:"disk_gb"`
}

type CreateVMResponse struct {
	VM VM `json:"vm"`
}

type ListVMResponse struct {
	Items []VM `json:"items"`
}

type DeleteVMResponse struct {
	ID    string  `json:"id"`
	State VMState `json:"state"`
}

type APIError struct {
	Error string `json:"error"`
}
