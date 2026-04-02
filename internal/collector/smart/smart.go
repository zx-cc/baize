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

type cmdTemplate struct {
	name string
	cmd  string
}

type Option struct {
	Type   string
	Block  string
	CtrlID string
	Did    string
}

// adding ` | grep -v ^$` to avoid situation where the returnCode is not zero.
var cmdTemplates = []cmdTemplate{
	{name: "megaraid", cmd: "/usr/sbin/smartctl -a -j /dev/bus/%s -d megaraid,%s | grep -v ^$"},
	{name: "cciss", cmd: "/usr/sbin/smartctl -a -j %s -d cciss,%s | grep -v ^$"},
	{name: "aacraid", cmd: "/usr/sbin/smartctl -a -j %s -d aacraid,%s | grep -v ^$"},
	{name: "nvme", cmd: "/usr/sbin/smartctl -a -j %s -d nvme | grep -v ^$"},
	{name: "jbod", cmd: "/usr/sbin/smartctl -a -j %s | grep -v ^$"},
}

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

func (s *SMART) parseNVMeInfo(data *smart) {
	s.ProtocolVer = data.NVMeVer.String
	s.SMARTAttrs = data.NVMeSmartHealth
}

func (s *SMART) parseSATAInfo(data *smart) {
	s.ProtocolVer = data.SATAVer.String
	s.SMARTAttrs = data.ATASmartAttributes.Table
}

func (s *SMART) parseSASInfo(data *smart) {
	s.ProtocolVer = data.SCSIVer
	s.SMARTAttrs = map[string]int64{
		"grown_defect_list": int64(data.ScsiGrownDefectList),
		"read_uce_errors":   data.ScsiErrorCounterLog.Read.TotalUncorrectedErrors,
		"write_uce_errors":  data.ScsiErrorCounterLog.Write.TotalUncorrectedErrors,
		"verify_uce_errors": data.ScsiErrorCounterLog.Verify.TotalUncorrectedErrors,
	}
}

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
