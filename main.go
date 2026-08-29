// this package will build a blocky binary that will drop most capabilities
// https://gokrazy.org/development/process-interface/index.html#privilege-dropping--security for reference
package main

import (
	"fmt"
	"golang.org/x/sys/unix"
	"log"
	"os"
	"os/exec"
	"syscall"
	"unsafe"

	"github.com/0xERR0R/blocky/cmd"
)

type capHeader struct {
	version uint32
	pid     int
}

type capData struct {
	effective   uint32
	permitted   uint32
	inheritable uint32
}

type caps struct {
	hdr  capHeader
	data [2]capData
}

func getCaps() (caps, error) {
	var c caps

	// Get capability version
	if _, _, errno := syscall.Syscall(syscall.SYS_CAPGET, uintptr(unsafe.Pointer(&c.hdr)), uintptr(unsafe.Pointer(nil)), 0); errno != 0 {
		return c, fmt.Errorf("SYS_CAPGET: %v", errno)
	}

	// Get current capabilities
	if _, _, errno := syscall.Syscall(syscall.SYS_CAPGET, uintptr(unsafe.Pointer(&c.hdr)), uintptr(unsafe.Pointer(&c.data[0])), 0); errno != 0 {
		return c, fmt.Errorf("SYS_CAPGET: %v", errno)
	}

	return c, nil
}

func mustDropPrivileges() {
	caps, err := getCaps()
	if err != nil {
		log.Fatalf("getCaps: %v", err)
	}

	// Add CAP_NET_BIND_SERVICE to permitted and inheritable so
	// it can be raised into the ambient capability set.
	caps.data[0].permitted |= 1 << unix.CAP_NET_BIND_SERVICE
	caps.data[0].inheritable |= 1 << unix.CAP_NET_BIND_SERVICE

	if _, _, errno := syscall.Syscall(
		syscall.SYS_CAPSET,
		uintptr(unsafe.Pointer(&caps.hdr)),
		uintptr(unsafe.Pointer(&caps.data[0])),
		0,
	); errno != 0 {
		log.Fatalf("SYS_CAPSET: %v", errno)
	}

	cmd := exec.Command(os.Args[0], os.Args[1:]...)
	cmd.Env = append(os.Environ(), "BLOCKY_PRIVILEGES_DROPPED=1")

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	cmd.SysProcAttr = &syscall.SysProcAttr{
		Credential: &syscall.Credential{
			Uid: 65534,
			Gid: 65534,
		},
		AmbientCaps: []uintptr{
			unix.CAP_NET_BIND_SERVICE,
		},
	}

	if err := cmd.Run(); err != nil {
		log.Fatal(err)
	}
	os.Exit(0)
}

func verifyPrivileges() {
	if os.Getuid() != 65534 {
		log.Fatalf("unexpected UID: %d", os.Getuid())
	}
	if os.Getgid() != 65534 {
		log.Fatalf("unexpected GID: %d", os.Getgid())
	}

	var hdr unix.CapUserHeader
	var data [2]unix.CapUserData

	hdr.Version = unix.LINUX_CAPABILITY_VERSION_3

	if err := unix.Capget(&hdr, &data[0]); err != nil {
		log.Fatalf("capget: %v", err)
	}

	mask := uint32(1) << unix.CAP_NET_BIND_SERVICE

	if data[0].Effective&mask == 0 {
		log.Fatal("CAP_NET_BIND_SERVICE is not effective")
	}

	if data[0].Permitted&mask == 0 {
		log.Fatal("CAP_NET_BIND_SERVICE is not permitted")
	}
}

func main() {
	if os.Getenv("BLOCKY_PRIVILEGES_DROPPED") != "1" {
		mustDropPrivileges()
	}
	verifyPrivileges()

	cmd.Execute()
}
