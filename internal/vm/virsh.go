package vm

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/xml"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
	"path/filepath"

	"github.com/project-kang/kang/pkg/types"
)

type VirshDriver struct {
	virshPath   string
	qemuImgPath string
	network     string
	imageDir    string
}

func NewVirshDriver(virshPath, qemuImgPath, network, imageDir string) *VirshDriver {
	if virshPath == "" {
		virshPath = "virsh"
	}

	if qemuImgPath == "" {
		qemuImgPath = "qemu-img"
	}

	if network == "" {
		network = "default"
	}

	if imageDir == "" {
		imageDir = "/var/lib/libvirt/images"
	}

	return &VirshDriver{
		virshPath:   virshPath,
		qemuImgPath: qemuImgPath,
		network:     network,
		imageDir:    imageDir,
	}
}

func (d *VirshDriver) Name() string {
	return "virsh"
}

func (d *VirshDriver) Create(ctx context.Context, req types.CreateVMRequest) (types.VM, error) {
	if req.Name == "" {
		return types.VM{}, fmt.Errorf("name is required")
	}

	if req.CPU <= 0 {
		return types.VM{}, fmt.Errorf("cpu must be greater than 0")
	}

	if req.MemoryMB <= 0 {
		return types.VM{}, fmt.Errorf("memory_mb must be greater than 0")
	}

	if req.BaseImagePath == "" {
		return types.VM{}, fmt.Errorf("base_image_path is required")
	}

	if req.DiskGB <= 0 {
		return types.VM{}, fmt.Errorf("disk_gb must be greater than 0")
	}

	if _, err := os.Stat(req.BaseImagePath); err != nil {
		return types.VM{}, fmt.Errorf("base_image_path is not accessible: %w", err)
	}

	id, err := shortID()
	if err != nil {
		return types.VM{}, err
	}

	domainName := "kang-" + id
	diskPath := filepath.Join(d.imageDir, domainName+".qcow2")

	if err := d.createDisk(ctx, req.BaseImagePath, diskPath, req.DiskGB); err != nil {
		return types.VM{}, err
	}

	domainXML := d.renderDomainXML(domainName, diskPath, req)

	tmp, err := os.CreateTemp("", domainName+"-*.xml")
	if err != nil {
		_ = os.Remove(diskPath)
		return types.VM{}, err
	}
	defer os.Remove(tmp.Name())

	if _, err := tmp.WriteString(domainXML); err != nil {
		_ = tmp.Close()
		_ = os.Remove(diskPath)
		return types.VM{}, err
	}

	if err := tmp.Close(); err != nil {
		_ = os.Remove(diskPath)
		return types.VM{}, err
	}

	if err := d.run(ctx, "define", tmp.Name()); err != nil {
		_ = os.Remove(diskPath)
		return types.VM{}, fmt.Errorf("virsh define failed: %w", err)
	}

	if err := d.run(ctx, "start", domainName); err != nil {
		_ = d.run(context.Background(), "undefine", domainName)
		_ = os.Remove(diskPath)
		return types.VM{}, fmt.Errorf("virsh start failed: %w", err)
	}

	return types.VM{
		ID:            id,
		Name:          req.Name,
		DomainName:    domainName,
		CPU:           req.CPU,
		MemoryMB:      req.MemoryMB,
		DiskGB:        req.DiskGB,
		BaseImagePath: req.BaseImagePath,
		DiskPath:      diskPath,
		State:         types.VMStateRunning,
		CreatedAt:     time.Now().UTC(),
	}, nil
}

func (d *VirshDriver) List(ctx context.Context) ([]types.VM, error) {
	out, err := d.output(ctx, "list", "--all", "--name")
	if err != nil {
		return nil, err
	}

	var vms []types.VM

	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		name := strings.TrimSpace(line)
		if name == "" || !strings.HasPrefix(name, "kang-") {
			continue
		}

		id := strings.TrimPrefix(name, "kang-")

		state := types.VMStateUnknown

		stateOut, err := d.output(ctx, "domstate", name)
		if err == nil {
			state = mapVirshState(stateOut)
		}

		cpu, memory := d.readDomainInfo(ctx, name)

		vms = append(vms, types.VM{
			ID:        id,
			Name:      name,
			CPU:       cpu,
			MemoryMB:  memory,
			State:     state,
			CreatedAt: time.Time{},
		})
	}

	return vms, nil
}

func (d *VirshDriver) createDisk(ctx context.Context, baseImagePath, diskPath string, diskGB int) error {
	if err := os.MkdirAll(d.imageDir, 0755); err != nil {
		return fmt.Errorf("failed to create image dir: %w", err)
	}

	size := fmt.Sprintf("%dG", diskGB)

	cmd := exec.CommandContext(
		ctx,
		d.qemuImgPath,
		"create",
		"-f", "qcow2",
		"-F", "qcow2",
		"-b", baseImagePath,
		diskPath,
		size,
	)

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("qemu-img create failed: %w: %s", err, strings.TrimSpace(stderr.String()))
	}

	return nil
}

func (d *VirshDriver) Delete(ctx context.Context, id string) error {
	if id == "" {
		return fmt.Errorf("id is required")
	}

	name := "kang-" + id
	diskPath := filepath.Join(d.imageDir, name+".qcow2")

	_ = d.run(ctx, "destroy", name)

	if err := d.run(ctx, "undefine", name); err != nil {
		return fmt.Errorf("virsh undefine failed: %w", err)
	}

	if err := os.Remove(diskPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove vm disk: %w", err)
	}

	return nil
}

func (d *VirshDriver) renderDomainXML(domainName, diskPath string, req types.CreateVMRequest) string {
	return fmt.Sprintf(`
<domain type='kvm'>
  <name>%s</name>
  <memory unit='MiB'>%d</memory>
  <vcpu>%d</vcpu>
  <os>
    <type arch='x86_64'>hvm</type>
    <boot dev='hd'/>
  </os>
  <features>
    <acpi/>
    <apic/>
  </features>
  <cpu mode='host-passthrough'/>
  <devices>
    <disk type='file' device='disk'>
      <driver name='qemu' type='qcow2'/>
      <source file='%s'/>
      <target dev='vda' bus='virtio'/>
    </disk>
    <interface type='network'>
      <source network='%s'/>
      <model type='virtio'/>
    </interface>
    <console type='pty'/>
    <graphics type='vnc' port='-1' autoport='yes'/>
  </devices>
</domain>
`,
		xmlEscape(domainName),
		req.MemoryMB,
		req.CPU,
		xmlEscape(diskPath),
		xmlEscape(d.network),
	)
}

func (d *VirshDriver) readDomainInfo(ctx context.Context, name string) (int, int) {
	out, err := d.output(ctx, "dominfo", name)
	if err != nil {
		return 0, 0
	}

	var cpu int
	var memoryMB int

	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)

		if strings.HasPrefix(line, "CPU(s):") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				cpu, _ = strconv.Atoi(fields[1])
			}
		}

		if strings.HasPrefix(line, "Max memory:") {
			fields := strings.Fields(line)
			if len(fields) >= 3 {
				kib, _ := strconv.Atoi(fields[2])
				memoryMB = kib / 1024
			}
		}
	}

	return cpu, memoryMB
}

func (d *VirshDriver) run(ctx context.Context, args ...string) error {
	cmd := exec.CommandContext(ctx, d.virshPath, args...)

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%w: %s", err, strings.TrimSpace(stderr.String()))
	}

	return nil
}

func (d *VirshDriver) output(ctx context.Context, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, d.virshPath, args...)

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("%w: %s", err, strings.TrimSpace(stderr.String()))
	}

	return stdout.String(), nil
}

func shortID() (string, error) {
	b := make([]byte, 4)

	if _, err := rand.Read(b); err != nil {
		return "", err
	}

	return hex.EncodeToString(b), nil
}

func mapVirshState(state string) types.VMState {
	state = strings.ToLower(strings.TrimSpace(state))

	switch state {
	case "running":
		return types.VMStateRunning
	case "shut off", "shutoff":
		return types.VMStateStopped
	default:
		return types.VMStateUnknown
	}
}

func xmlEscape(value string) string {
	var buf bytes.Buffer
	_ = xml.EscapeText(&buf, []byte(value))
	return buf.String()
}
