package cpu

import (
	"bytes"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/zx-cc/baize/internal/collector/smbios"
	"github.com/zx-cc/baize/pkg/shell"
	"github.com/zx-cc/baize/pkg/utils"
)

func New() *CPU {
	return &CPU{
		PowerState: "PowerSave",
		Diagnose:   "Healthy",
	}
}

func (c *CPU) Collect() error {
	var errs []error

	if err := c.collectFromLscpu(); err != nil {
		errs = append(errs, err)
	}

	if err := c.collectFromSMBIOS(); err != nil {
		errs = append(errs, err)
	}

	if err := c.collectThreads(); err != nil {
		errs = append(errs, err)
	}

	c.associate()

	return errors.Join(errs...)
}

// socketIDMap maps common socket designation strings to normalized zero-based
// socket index strings, enabling consistent cross-platform CPU socket matching.
var (
	socketIDMap = map[string]string{
		"P0": "0", "Proc 1": "0", "CPU 1": "0", "CPU01": "0", "CPU1": "0", "Socket 1": "0",
		"P1": "1", "Proc 2": "1", "CPU 2": "1", "CPU02": "1", "CPU2": "1", "Socket 2": "1",
	}
)

func (c *CPU) associate() {
	var errs []error

	tempMap, err := collectTempFromHwmon()
	if err != nil {
		return
	}

	for _, entry := range c.PhysicalCPUs {
		// Resolve the socket designation to a normalized physical ID index.
		id, ok := socketIDMap[entry.Designation]
		if !ok {
			errs = append(errs, errors.New("socket designation not found"))
			continue
		}
		for _, thread := range c.threads {

			println(id, thread.PID)

			if id != thread.PID {
				continue
			}

			if strings.HasPrefix(c.Architecture, "x86") {
				switch c.Vendor {
				case "Intel":
					thread.Temp = fmt.Sprintf("%d ℃", tempMap[thread.PID+"-"+thread.CID])
				case "AMD":
					thread.Temp = fmt.Sprintf("%d ℃", tempMap[thread.PID+"-"+thread.DID])
				}
			}

			entry.Threads = append(entry.Threads, thread)
		}
	}
}

func (c *CPU) collectFromSMBIOS() error {
	cpus, err := smbios.GetTypeData[*smbios.Type4Processor](4)
	if err != nil {
		return err
	}

	for _, cpu := range cpus {
		c.PhysicalCPUs = append(c.PhysicalCPUs, &PhysicalCPU{
			Designation:     cpu.SocketDesignation,
			Type:            cpu.ProcessorType.String(),
			Family:          cpu.GetFamily().String(),
			Vendor:          cpu.Manufacturer,
			Version:         cpu.Version,
			ExternalClock:   strconv.Itoa(int(cpu.ExternalClock)) + " MHz",
			Status:          cpu.Status.String(),
			Voltage:         fmt.Sprintf("%.2f v", cpu.GetVoltage()),
			Upgrade:         cpu.ProcessorUpgrade.String(),
			CoreCount:       strconv.Itoa(cpu.GetCoreCount()),
			CoreEnabled:     strconv.Itoa(cpu.GetCoreEnabled()),
			ThreadCount:     strconv.Itoa(cpu.GetThreadCount()),
			Characteristics: cpu.Characteristics.StringList(),
		})
	}

	return nil
}

var (
	vendorMap = map[string]string{
		"AuthenticAMD":         "AMD",
		"GenuineIntel":         "Intel",
		"Intel(R) Corporation": "Intel",
		"0x48":                 "HiSilicon",
	}
)

type fieldSetter func(*CPU, string)

var lscpuFieldSetters = map[string]fieldSetter{
	"Architecture":        func(info *CPU, value string) { info.Architecture = value },
	"Byte Order":          func(info *CPU, value string) { info.ByteOrder = value },
	"Address sizes":       func(info *CPU, value string) { info.AddressSize = value },
	"CPU family":          func(info *CPU, value string) { info.Family = value },
	"Model":               func(info *CPU, value string) { info.Model = value },
	"Model name":          func(info *CPU, value string) { info.ModelName = value },
	"Stepping":            func(info *CPU, value string) { info.Stepping = value },
	"BogoMIPS":            func(info *CPU, value string) { info.BogoMIPS = value },
	"Virtualization":      func(info *CPU, value string) { info.Virtualization = value },
	"L1d cache":           func(info *CPU, value string) { info.L1dCache = value },
	"L1i cache":           func(info *CPU, value string) { info.L1iCache = value },
	"L2 cache":            func(info *CPU, value string) { info.L2Cache = value },
	"L3 cache":            func(info *CPU, value string) { info.L3Cache = value },
	"CPU(s)":              func(info *CPU, value string) { info.CPUs = value },
	"On-line CPU(s) list": func(info *CPU, value string) { info.OnlineCPUs = value },
	"Thread(s) per core":  func(info *CPU, value string) { info.ThreadsPerCore = value },
	"Core(s) per socket":  func(info *CPU, value string) { info.CoresPerSocket = value },
	"Socket(s)":           func(info *CPU, value string) { info.Sockets = value },
	"Vendor ID": func(info *CPU, value string) {
		if vendor, ok := vendorMap[value]; ok {
			info.Vendor = vendor
		} else {
			info.Vendor = value
		}
	},
	"CPU op-mode(s)": func(info *CPU, value string) { info.OpMode = value },
	"Flags":          func(info *CPU, value string) { info.Flags = strings.Fields(value) },
}

func (c *CPU) collectFromLscpu() error {
	data, err := shell.Run("lscpu")
	if err != nil {
		return err
	}

	scanner := utils.NewScanner(bytes.NewReader(data))
	for {
		k, v, isEnd := scanner.ParseLine(":")
		if isEnd {
			break
		}

		if setter, ok := lscpuFieldSetters[k]; ok {
			setter(c, v)
		}
	}

	return scanner.Err()
}

func (c *CPU) Name() string {
	return "CPU"
}

func (c *CPU) Marshal() {

}

func (c *CPU) PrintDetail() {}

func (c *CPU) PrintBreif() {}
