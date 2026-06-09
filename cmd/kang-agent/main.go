package main

import (
	"flag"
	"log"
	"net/http"

	"github.com/project-kang/kang/internal/agent"
	"github.com/project-kang/kang/internal/vm"
)

func main() {
	listenAddr := flag.String("listen", ":8081", "HTTP listen address")
	hostID := flag.String("host-id", "local-dev", "unique Kang host ID")
	vmDriver := flag.String("vm-driver", "virsh", "VM driver: virsh")
	virshPath := flag.String("virsh-path", "virsh", "path to virsh binary")
	libvirtNetwork := flag.String("libvirt-network", "default", "libvirt network name")
	qemuImgPath := flag.String("qemu-img-path", "qemu-img", "path to qemu-img binary")
        imageDir := flag.String("image-dir", "/var/lib/libvirt/images", "directory where Kang VM disks are stored")

	flag.Parse()

	var driver vm.Driver

	switch *vmDriver {
	case "virsh":
		driver = vm.NewVirshDriver(*virshPath, *qemuImgPath, *libvirtNetwork, *imageDir)
	default:
		log.Fatalf("unsupported vm-driver: %s", *vmDriver)
	}

	srv := agent.NewServer(*hostID, driver)

	log.Printf(
		"starting kang-agent host_id=%s listen=%s vm_driver=%s",
		*hostID,
		*listenAddr,
		driver.Name(),
	)

	if err := http.ListenAndServe(*listenAddr, srv.Routes()); err != nil {
		log.Fatalf("kang-agent failed: %v", err)
	}
}
