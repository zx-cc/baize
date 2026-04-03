// Package product — server_info.go queries SMBIOS tables for server platform
// identity information (BIOS, system, baseboard, chassis).
package product

import (
	"strconv"
	"strings"

	"github.com/zx-cc/baize/internal/collector/smbios"
)

// collectBIOS reads SMBIOS type-0 and returns a populated BIOS struct.
func collectBIOS() (*BIOS, error) {
	bios, err := smbios.GetTypeData[*smbios.Type0BIOS](0)
	if err != nil {
		return nil, err
	}

	entry := bios[0]

	return &BIOS{
		Vendor:           entry.Vendor,
		Version:          entry.Version,
		ReleaseDate:      entry.ReleaseDate,
		BIOSRevision:     formatRevision(entry.BIOSMajorRelease, entry.BIOSMinorRelease),
		FirmwareRevision: formatRevision(entry.ECMajorRelease, entry.ECMinorRelease),
	}, nil
}

// collectSystem reads SMBIOS type-1 and returns a populated System struct.
func collectSystem() (*System, error) {
	system, err := smbios.GetTypeData[*smbios.Type1System](1)
	if err != nil {
		return nil, err
	}

	entry := system[0]

	return &System{
		Manufacturer: entry.Manufacturer,
		ProductName:  entry.ProductName,
		SN:           entry.SerialNumber,
		UUID:         entry.UUID.String(),
		PN:           entry.SKU,
		Family:       entry.Family,
	}, nil
}

// collectBaseBoard reads SMBIOS type-2 and returns a populated BaseBoard struct.
func collectBaseBoard() (*BaseBoard, error) {
	baseboard, err := smbios.GetTypeData[*smbios.Type2BaseBoard](2)
	if err != nil {
		return nil, err
	}

	entry := baseboard[0]

	return &BaseBoard{
		Manufacturer: entry.Manufacturer,
		SN:           entry.SerialNumber,
		Type:         entry.BoardType.String(),
	}, nil
}

// collectChassis reads SMBIOS type-3 and returns a populated Chassis struct.
func collectChassis() (*Chassis, error) {
	chassis, err := smbios.GetTypeData[*smbios.Type3Chassis](3)
	if err != nil {
		return nil, err
	}

	entry := chassis[0]

	return &Chassis{
		Manufacturer: entry.Manufacturer,
		Type:         entry.ChassisType.String(),
		SN:           entry.SerialNumber,
		AssetTag:     entry.AssetTag,
		Height:       formatChassisHeight(entry.ChassisType.String(), entry.Height),
		PN:           entry.SKU,
	}, nil
}

// formatRevision formats a major.minor byte pair as a "M.N" version string.
func formatRevision(major, minor uint8) string {
	buf := make([]byte, 0, 8)
	buf = strconv.AppendUint(buf, uint64(major), 10)
	buf = append(buf, '.')
	buf = strconv.AppendUint(buf, uint64(minor), 10)
	return string(buf)
}

// formatChassisHeight returns the chassis height in rack units (e.g. "2U")
// for rack-mounted chassis types; returns an empty string for other types.
func formatChassisHeight(chassisType string, height uint8) string {
	if !strings.HasPrefix(chassisType, "Rack") {
		return ""
	}

	return strconv.Itoa(int(height)) + "U"
}
