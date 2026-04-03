// Package smart provides a vendor-neutral interface for collecting SMART
// (Self-Monitoring, Analysis, and Reporting Technology) data from storage
// devices via smartctl.
//
// It supports four device access modes:
//   - megaraid — drives behind Broadcom/LSI MegaRAID controllers
//   - cciss    — drives behind HPE Smart Array (cciss) controllers
//   - aacraid  — drives behind Microchip/Adaptec controllers
//   - nvme     — direct-attached NVMe drives
//   - jbod     — direct-attached SATA/SAS drives (pass-through / JBOD)
//
// Protocol-specific attributes (SATA ATA SMART table, SAS error counters,
// NVMe health log) are decoded into a unified SMART result struct.
package smart

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/zx-cc/baize/pkg/shell"
	"github.com/zx-cc/baize/pkg/utils"
)

// cmdTemplate pairs a device-access mode name with its formatted smartctl
// command template string.
type cmdTemplate struct {
	name string
	cmd  string
}

// Option carries the parameters required to build a smartctl command for a
// specific storage device.
type Option struct {
	Type   string // device access mode: "megaraid", "cciss", "aacraid", "nvme", "jbod"
	Block  string // block device path (e.g. "/dev/sda", "/dev/nvme0")
	CtrlID string // controller ID (used by megaraid and aacraid)
	Did    string // device ID within the controller (slot / port number)
}

// cmdTemplates is the ordered list of supported smartctl command templates.
// Each template appends `| grep -v ^$` to suppress blank lines that would
// otherwise cause JSON parsing failures when smartctl exits with a non-zero
// status (e.g. when SMART is not supported).
var cmdTemplates = []cmdTemplate{
	{name: "megaraid", cmd: "/usr/sbin/smartctl -a -j /dev/bus/%s -d megaraid,%s | grep -v ^$"},
	{name: "cciss", cmd: "/usr/sbin/smartctl -a -j %s -d cciss,%s | grep -v ^$"},
	{name: "aacraid", cmd: "/usr/sbin/smartctl -a -j %s -d aacraid,%s | grep -v ^$"},
	{name: "nvme", cmd: "/usr/sbin/smartctl -a -j %s -d nvme | grep -v ^$"},
	{name: "jbod", cmd: "/usr/sbin/smartctl -a -j %s | grep -v ^$"},
}

// GetSmartctlData runs smartctl for the given Option, parses the JSON output,
// and returns a populated SMART result.  Both SMART attribute data and
// write/read-cache status are included in the result.
func GetSmartctlData(option Option) (*SMART, error) {
	if option.Type == "" {
		return nil, fmt.Errorf("raid type is empty")
	}

	cmd, err := getSmartctlCommand(option)
	if err != nil {
		return nil, err
	}

	output, err := shell.RunShell(cmd)
	if err != nil {
		return nil, err
	}

	data, err := parseSMARTData(output, cmd)

	return data, err
}

// getSmartctlCommand selects and formats the smartctl command template that
// matches option.Type, substituting the appropriate block device, controller
// ID, and device ID placeholders.
func getSmartctlCommand(option Option) (string, error) {
	var cmd string
	for _, c := range cmdTemplates {
		switch c.name {
		case "megaraid":
			cmd = fmt.Sprintf(c.cmd, option.CtrlID, option.Did)
		case "cciss":
			cmd = fmt.Sprintf(c.cmd, option.Block, option.Did)
		case "aacraid":
			cmd = fmt.Sprintf(c.cmd, option.CtrlID, option.Did)
		case "nvme":
			cmd = fmt.Sprintf(c.cmd, option.Block)
		case "jbod":
			cmd = fmt.Sprintf(c.cmd, option.Block)
		}

		if c.name == option.Type {
			return cmd, nil
		}
	}

	return "", fmt.Errorf("not found smartctl command")
}

// parseSMARTData unmarshals the raw smartctl JSON output into an internal
// smart struct, extracts the common base information, and fetches cache
// policy settings with a secondary smartctl invocation.
func parseSMARTData(data []byte, cmd string) (*SMART, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("SMART data is empty")
	}

	var s smart
	err := json.Unmarshal(data, &s)
	if err != nil {
		return nil, err
	}

	res := &SMART{}
	res.parseBaseInfo(&s)
	err = res.getWriteAndReadCache(cmd)

	return res, nil
}

// parseBaseInfo populates the protocol-agnostic fields of a SMART result from
// the raw smartctl JSON and dispatches to the appropriate protocol-specific
// parser (NVMe, SATA, or SAS).
func (s *SMART) parseBaseInfo(data *smart) {
	s.ModelName = data.ModelName
	s.SN = data.SerialNumber
	s.SMARTStatus = data.SmartStatus.Passed
	s.PowerOn = strconv.Itoa(data.PowerOnTime.Hours) + " h"
	s.Temperature = strconv.Itoa(data.Temperature.Current) + " °C"

	// NVMe: no form_factor,default 2.5 inchs
	if data.FormFactor == nil {
		s.FormFactor = "2.5 inchs"
	} else {
		s.FormFactor = data.FormFactor.Name
	}

	// sas: revision
	s.Firmware = data.FirmwareVersion
	if s.Firmware == "" {
		s.Firmware = data.Revision
	}

	// determine sdd or hdd by rotation rate
	if data.RotationRate == 0 {
		s.Rotation = "SSD"
		s.MediaType = "SSD"
	} else {
		s.Rotation = strconv.Itoa(data.RotationRate) + " RPM"
		s.MediaType = "HDD"
	}

	// capacity: NVMeTotalCap、UserCapacity(sas/sata)
	if data.UserCapacity == nil {
		s.Capacity = utils.AutoFormatSize(zeroAfterFirstZero(data.NVMeTotalCap), "B", false)
	} else {
		s.Capacity = utils.AutoFormatSize(zeroAfterFirstZero(data.UserCapacity.Bytes), "B", false)
	}

	// model_name: SAMSUNG MZQL2960HCJR-00B7C、Micron_5300_MTFDDAK960TDS
	var parts []string
	if strings.Contains(s.ModelName, "_") {
		parts = strings.Split(s.ModelName, "_")
	} else {
		parts = strings.Fields(s.ModelName)
	}
	s.Vendor = parts[0]
	s.PN = parts[len(parts)-1]

	// determine protocol
	switch strings.ToLower(data.Device.Protocol) {
	case "nvme":
		s.Protocol = "NVMe"
		s.parseNVMeInfo(data)
	case "ata", "sata":
		s.Protocol = "SATA"
		s.parseSATAInfo(data)
	case "scsi", "sas":
		s.Protocol = "SAS"
		s.parseSASInfo(data)
	}
}

// parseNVMeInfo copies NVMe-specific fields (version, smart health log) from
// the raw smartctl JSON into the SMART result.
func (s *SMART) parseNVMeInfo(data *smart) {
	s.ProtocolVer = data.NVMeVer.String
	s.SMARTAttrs = data.NVMeSmartHealth
}

// parseSATAInfo copies SATA-specific fields (SATA version, ATA SMART attribute
// table) from the raw smartctl JSON into the SMART result.
func (s *SMART) parseSATAInfo(data *smart) {
	s.ProtocolVer = data.SATAVer.String
	s.SMARTAttrs = data.ATASmartAttributes.Table
}

// parseSASInfo copies SAS-specific fields (SCSI version, grown defect list,
// and uncorrected error counters) from the raw smartctl JSON into the SMART
// result.
func (s *SMART) parseSASInfo(data *smart) {
	s.ProtocolVer = data.SCSIVer
	s.SMARTAttrs = map[string]int64{
		"grown_defect_list": int64(data.ScsiGrownDefectList),
		"read_uce_errors":   data.ScsiErrorCounterLog.Read.TotalUncorrectedErrors,
		"write_uce_errors":  data.ScsiErrorCounterLog.Write.TotalUncorrectedErrors,
		"verify_uce_errors": data.ScsiErrorCounterLog.Verify.TotalUncorrectedErrors,
	}
}

// getWriteAndReadCache runs `smartctl -g all` (derived from the original -a -j
// command) and parses the write-cache and read-cache status lines.
func (s *SMART) getWriteAndReadCache(cmd string) error {
	cmd = strings.ReplaceAll(cmd, "-a -j", "-g all")
	output, err := shell.RunShell(cmd)
	if err != nil {
		return err
	}

	scanner := utils.NewScanner(bytes.NewReader(output))
	for {
		k, v, ended := scanner.ParseLine(":")
		if ended {
			break
		}
		switch k {
		case "Writeback Cache is", "Write Cache is":
			s.WriteCache = v
		case "Read Cache is", "Rd look-ahead is":
			s.ReadCache = v
		}
	}

	return scanner.Err()
}

// zeroAfterFirstZero converts a raw NVMe capacity integer to a meaningful
// float64 value by zeroing all digits after the first zero digit.
// smartctl encodes NVMe capacity as a decimal where trailing zeros beyond
// the first zero are padding (e.g. 960197124096 → 960000000000).
func zeroAfterFirstZero(n int64) float64 {
	s := strconv.FormatInt(n, 10)
	b := []byte(s)

	firstZero := false
	for i, c := range b {
		if firstZero {
			b[i] = '0'
		} else if c == '0' {
			firstZero = true
		}
	}

	f, _ := strconv.ParseFloat(string(b), 64)

	return f
}
