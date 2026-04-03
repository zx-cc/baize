// Package memory collects memory hardware and runtime statistics from three
// independent sources:
//   - /proc/meminfo   — kernel-reported runtime memory usage counters
//   - SMBIOS type-17  — physical DIMM slot inventory from system firmware
//   - /sys/bus/edac    — hardware ECC error counters via the EDAC kernel subsystem
//
// After collection, a health diagnosis is performed to detect asymmetric
// configurations, slot mismatches, and potential DIMM failures.
package memory

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/zx-cc/baize/internal/collector/smbios"
	"github.com/zx-cc/baize/pkg/paths"
	"github.com/zx-cc/baize/pkg/utils"
)

// New returns an initialised Memory collector with pre-allocated slices.
func New() *Memory {
	return &Memory{
		PhysicalMemoryEntries: make([]*SmbiosMemoryEntry, 0, 32),
		EdacMemoryEntries:     make([]*EdacMemoryEntry, 0, 32),
	}
}

// Collect runs all memory sub-collectors in sequence and joins any errors.
func (m *Memory) Collect() error {
	errs := make([]error, 0, 3)

	// Collect runtime memory statistics from /proc/meminfo.
	if err := m.collectFromMeminfo(); err != nil {
		errs = append(errs, err)
	}

	// Collect physical DIMM information from SMBIOS type-17 tables.
	if err := m.collectFromSMBIOS(); err != nil {
		errs = append(errs, err)
	}

	// Collect EDAC memory error counters from /sys/bus/edac/devices.
	if err := m.collectFromEdac(); err != nil {
		errs = append(errs, err)
	}

	// Run health checks and populate Diagnose / DiagnoseDetail fields.
	m.diagnose()

	return errors.Join(errs...)
}

// collectFromMeminfo reads /proc/meminfo and parses each field into a
// human-readable size string (auto-scaled with binary prefixes).
func (m *Memory) collectFromMeminfo() error {
	file, err := os.Open(paths.ProcMeminfo)
	if err != nil {
		return fmt.Errorf("open %s: %w", paths.ProcMeminfo, err)
	}
	defer file.Close()

	fieldsMap := map[string]*string{
		"MemTotal":        &m.MemTotal,
		"MemFree":         &m.MemFree,
		"MemAvailable":    &m.MemAvailable,
		"SwapCached":      &m.SwapCached,
		"SwapTotal":       &m.SwapTotal,
		"SwapFree":        &m.SwapFree,
		"Buffers":         &m.Buffer,
		"Cached":          &m.Cached,
		"Slab":            &m.Slab,
		"SReclaimable":    &m.SReclaimable,
		"SUnreclaim":      &m.SUnreclaim,
		"KReclaimable":    &m.KReclaimable,
		"KernelStack":     &m.KernelStack,
		"PageTables":      &m.PageTables,
		"Dirty":           &m.Dirty,
		"Writeback":       &m.Writeback,
		"HugePages_Total": &m.HPagesTotal,
		"HugePagessize":   &m.HPageSize,
		"Hugetlb":         &m.HugeTlb,
	}
	scanner := utils.NewScanner(file)
	for {
		k, v, isEnd := scanner.ParseLine(":")
		if isEnd {
			break
		}

		if ptr, exists := fieldsMap[k]; exists {
			num, unit, ok := strings.Cut(v, " ")
			if !ok {
				*ptr = v
				continue
			}
			if val, err := strconv.ParseFloat(num, 64); err == nil {
				*ptr = utils.AutoFormatSize(val, unit, true)
			}
		}
	}

	return scanner.Err()
}

// collectFromSMBIOS queries SMBIOS type-17 tables to build the physical DIMM
// inventory, computing the total installed physical memory size.
func (m *Memory) collectFromSMBIOS() error {
	memoryTables, err := smbios.GetTypeData[*smbios.Type17MemoryDevice](17)
	if err != nil {
		return err
	}

	bitWidthStr := func(v uint16) string {
		if v == 0 || v == 0xFFFF {
			return "Unknown"
		}
		return fmt.Sprintf("%d bits", v)
	}

	speedStr := func(v uint16) string {
		if v == 0 || v == 0xFFFF {
			return "Unknown"
		}
		return fmt.Sprintf("%d MT/s", v)
	}

	voltageStr := func(v uint16) string {
		switch {
		case v == 0:
			return "Unknown"
		case v%100 == 0:
			return fmt.Sprintf("%.1f V", float32(v)/1000.0)
		default:
			return fmt.Sprintf("%g V", float32(v)/1000.0)
		}
	}

	m.Maxslots = strconv.Itoa(len(memoryTables))
	var totalSize int

	for _, t := range memoryTables {
		speed := speedStr(t.Speed)
		if speed == "Unknown" {
			continue
		}

		m.PhysicalMemoryEntries = append(m.PhysicalMemoryEntries, &SmbiosMemoryEntry{
			Size:              t.GetSizeString(),
			SerialNumber:      t.SerialNumber,
			Manufacturer:      t.Manufacturer,
			TotalWidth:        bitWidthStr(t.TotalWidth),
			DataWidth:         bitWidthStr(t.DataWidth),
			FormFactor:        t.FormFactor.String(),
			DeviceLocator:     t.DeviceLocator,
			BankLocator:       t.BankLocator,
			Type:              t.Type.String(),
			TypeDetail:        t.TypeDetail.String(),
			Speed:             speed,
			PartNumber:        t.PartNumber,
			Rank:              t.GetRankString(),
			ConfiguredSpeed:   speedStr(t.ConfiguredSpeed),
			ConfiguredVoltage: voltageStr(t.ConfiguredVoltage),
			Technology:        t.Technology.String(),
		})

		if size, err := toBytes(t.GetSizeString()); err == nil {
			totalSize += size
		}
	}

	m.PhysicalMemorySize = utils.AutoFormatSize(float64(totalSize), "B", true)
	m.UsedSlots = strconv.Itoa(len(m.PhysicalMemoryEntries))

	return nil
}

// diagnose performs sanity checks on the collected memory data and populates
// Diagnose and DiagnoseDetail fields with any detected anomalies.
func (m *Memory) diagnose() {
	var msg []string

	// Check for slot count mismatch between SMBIOS and EDAC.
	if m.EdacSlots != "" && m.EdacSlots != m.UsedSlots {
		msg = append(msg, "SMBIOS and EDAC memory slots are not equal")
	}

	// Check if the OS-visible memory size diverges from SMBIOS physical size
	// by more than one DIMM's worth (which may indicate a failed/missing module).
	if len(m.PhysicalMemoryEntries) > 0 {
		sysSize, sysErr := toBytes(m.MemTotal)
		smbiosSize, smbiosErr := toBytes(m.PhysicalMemorySize)
		if sysErr == nil && smbiosErr == nil {
			if smbiosSize-sysSize > smbiosSize/len(m.PhysicalMemoryEntries) {
				msg = append(msg, "has unhealthy memory")
			}
		}
	}

	// Warn when DIMM count is odd, which typically indicates an asymmetric configuration.
	if len(m.PhysicalMemoryEntries)%2 != 0 {
		msg = append(msg, "memory count should be even")
	}

	if len(msg) != 0 {
		m.Diagnose = "Unhealthy"
		m.DiagnoseDetail = strings.Join(msg, "; ")
	}
}

// toBytes converts a human-readable size string (e.g. "16 GB") to bytes.
// Supported units: B, KB, MB, GB, TB (case-insensitive).
func toBytes(s string) (int, error) {
	parts := strings.Fields(s)
	if len(parts) != 2 {
		return 0, fmt.Errorf("invalid size string: %s", s)
	}

	unit := strings.ToLower(parts[1])
	res, err := strconv.Atoi(parts[0])

	switch unit {
	case "b":
		return res, err
	case "kb":
		return res * 1024, err
	case "mb":
		return res * 1024 * 1024, err
	case "gb":
		return res * 1024 * 1024 * 1024, err
	case "tb":
		return res * 1024 * 1024 * 1024 * 1024, err
	}

	return res, err
}

// Name returns the module identifier used for routing by the collector manager.
func (m *Memory) Name() string {
	return "MEMORY"
}

// Jprintln serialises the collected memory data to JSON and writes it to stdout.
func (m *Memory) Jprintln() error {
	return utils.JSONPrintln(m)
}

// Sprintln prints a brief memory summary to stdout.
func (m *Memory) Sprintln() {
	utils.PrinterInstance.Print(m, "MEMORY INFO", "brief")
}

// Lprintln prints a detailed memory report to stdout.
func (m *Memory) Lprintln() {
	utils.PrinterInstance.Print(m, "MEMORY INFO", "detail")
}
