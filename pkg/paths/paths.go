package paths

import (
	"fmt"
	"os"
)

func Stat(path string) error {
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("path %s not exist: %w", path, err)
		}

		return fmt.Errorf("accessing path %s: %w", path, err)
	}
	return nil
}

func Exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil || os.IsExist(err)
}

const (
	DevMem     = "/dev/mem"
	LibModules = "/lib/modules"

	EtcOsRelease     = "/etc/os-release"
	EtcCentosRelease = "/etc/centos-release"
	EtcRedhatRelease = "/etc/redhat-release"
	EtcRockyRelease  = "/etc/rocky-release"
	EtcDebianVersion = "/etc/debian_version"

	ProcKernelHostname  = "/proc/sys/kernel/hostname"
	ProcKernelOstype    = "/proc/sys/kernel/ostype"
	ProcKernelOsrelease = "/proc/sys/kernel/osrelease"
	ProcKernelVersion   = "/proc/sys/kernel/version"

	SysBusPciDevices     = "/sys/bus/pci/devices"
	SysModule            = "/sys/module"
	SysFirmwareDmiTables = "/sys/firmware/dmi/tables"
)
