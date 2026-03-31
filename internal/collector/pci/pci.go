package pci

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/zx-cc/baize/pkg/paths"
	"github.com/zx-cc/baize/pkg/shell"
	"github.com/zx-cc/baize/pkg/utils"
)

// Collect gathers all PCI devices
func Collect() ([]*PCI, error) {
	entries, err := os.ReadDir(paths.SysBusPciDevices)
	if err != nil {
		return nil, fmt.Errorf("read pci devices: %w", err)
	}

	devices := make([]*PCI, 0, len(entries))
	for _, entry := range entries {
		p, err := GetByBus(entry.Name())
		if err != nil {
			continue
		}
		devices = append(devices, p)
	}

	return devices, nil
}

// GetByBus get device infomation by pci bus address.
// bus format example: "0000:3b:00.0" or "3b:00.1"
func GetByBus(bus string) (*PCI, error) {
	bus = normalizeBusAddr(bus)

	devicePath := filepath.Join(paths.SysBusPciDevices, bus)
	if !paths.Exists(devicePath) {
		return nil, fmt.Errorf("PCI device %s not found", bus)
	}

	p := &PCI{Bus: bus}

	collectBasicInfo(p)
	p.Driver = collectDriverInfo(devicePath)
	p.Link = collectLinkInfo(bus)

	return p, nil
}

// normalizeBusAddr normalizes the PCI bus address to a consistent format.
func normalizeBusAddr(bus string) string {
	bus = strings.TrimSpace(bus)
	if len(bus) > 0 && !strings.Contains(bus[:min(5, len(bus))], ":") {
		return bus
	}

	parts := strings.Split(bus, ":")
	if len(parts) == 2 {
		return "0000:" + bus
	}

	return bus
}

// collectLinkInfo collects PCI basic information.
// path: /sys/bus/pci/devices/0000:3b:01.1
// basic info: vendor, device, subsystem_vendor, subsystem_device...
func collectBasicInfo(p *PCI) {
	fields := []struct {
		name   string
		target string
	}{
		{name: "vendor", target: p.VendorID},
		{name: "device", target: p.DeviceID},
		{name: "subsystem_vendor", target: p.SubVendorID},
		{name: "subsystem_device", target: p.SubDeviceID},
		{name: "Revision", target: p.Revision},
		{name: "class", target: p.ClassID},
	}

	for _, field := range fields {
		file := filepath.Join(paths.SysBusPciDevices, field.name)
		content, err := utils.ReadLine(file)
		if err != nil {
			continue
		}

		content = strings.TrimPrefix(content, "0x")
		if field.name == "class" {
			if len(content) >= 4 {
				field.target = content[:4]
				p.SubClass = content[2:4]
			}
		} else {
			field.target = content
		}
	}

	resolveNamesFromPCIIDS(p)
	resolveNamesFromLspci(p)
}

func resolveNamesFromPCIIDS(p *PCI) bool {
	db := GetPCIDatabase()
	if !db.IsLoaded() {
		return false
	}

	resolved := false
	if vendorName := db.GetVendorName(p.VendorID); vendorName != "" {
		p.Vendor = vendorName
		resolved = true
	}

	// 获取设备名称
	if deviceName := db.GetDeviceName(p.VendorID, p.DeviceID); deviceName != "" {
		p.Device = deviceName
		resolved = true
	}

	// 获取类别名称
	if p.ClassID != "" {
		classID := p.ClassID
		if len(classID) >= 2 {
			// ClassID 格式可能是 "0280"，前两位是类别，后两位是子类别
			mainClassID := classID[:2]

			if className := db.GetClassName(mainClassID); className != "" {
				p.Class = className
				resolved = true
			}

			// 获取子类别名称
			if len(classID) >= 4 {
				subclassID := classID[2:4]
				if subclassName := db.GetSubclassName(mainClassID, subclassID); subclassName != "" {
					p.SubClass = subclassName
					resolved = true
				}
			}
		}
	}

	return resolved
}

// resolveNamesFromLspci
func resolveNamesFromLspci(p *PCI) {
	output, err := shell.Run("lspci", "-vmms", p.Bus)
	if err != nil {
		return
	}

	scanner := utils.NewScanner(bytes.NewReader(output))
	for {
		k, v, ended := scanner.ParseLine(":")
		if ended {
			break
		}
		switch k {
		case "Vendor":
			if p.Vendor == "" {
				p.Vendor = v
			}
		case "Device":
			if p.Device == "" {
				p.Device = v
			}
		case "Class":
			if p.Class == "" {
				p.Class = v
			}
		case "SVendor":
			if p.SubVendor == "" {
				p.SubVendor = v
			}
		case "SDevice":
			if p.SubDevice == "" {
				p.SubDevice = v
			}
		}
	}
}

// collectDriverInfo collects PCI driver information.
func collectDriverInfo(path string) *Driver {
	link := filepath.Join(path, "driver")
	linkPath, err := os.Readlink(link)
	if err != nil {
		return nil
	}

	driverName := filepath.Base(linkPath)
	driver := &Driver{DriverName: driverName}

	moudlePath := filepath.Join(paths.SysModule, driverName)
	if ver, err := utils.ReadLine(filepath.Join(moudlePath, "version")); err == nil {
		driver.DriverVer = ver
	}
	if srcVer, err := utils.ReadLine(filepath.Join(moudlePath, "srcversion")); err == nil {
		driver.SrcVer = srcVer
	}

	driver.FileName = findModulePath(driverName)
	return driver
}

// findModulePath finds the module path for a given driver name.
func findModulePath(driverName string) string {
	kernel, err := shell.Run("uname", "-r")
	if err != nil {
		return ""
	}

	searchPath := filepath.Join(paths.SysModule, strings.TrimSpace(string(kernel)), "kernel")
	var modulePath string
	filepath.Walk(searchPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			return nil
		}

		baseName := filepath.Base(path)
		if strings.HasPrefix(baseName, driverName+".ko") ||
			strings.HasPrefix(baseName, driverName+".") && strings.Contains(baseName, ".ko") {
			modulePath = path
			return filepath.SkipAll
		}
		return nil
	})

	return modulePath
}

// collectLinkInfo collects PCI link information.
func collectLinkInfo(path string) *Link {
	link := &Link{}
	fields := []struct {
		name   string
		target string
	}{
		{name: "max_link_speed", target: link.MaxSpeed},
		{name: "max_link_width", target: link.MaxWidth},
		{name: "current_link_speed", target: link.CurrSpeed},
		{name: "current_link_width", target: link.CurrWidth},
	}

	for _, field := range fields {
		if content, err := utils.ReadLine(filepath.Join(path, field.name)); err == nil {
			field.target = content
		}
	}

	return link
}
