// Package raid — microchip_adaptec.go collects storage controller and drive
// data from Microchip/Adaptec RAID controllers using the arcconf utility.
//
// Note: On HP platforms (detected via `dmidecode -s system-manufacturer`),
// this file delegates to the HPE collector rather than using arcconf.
//
// Discovery flow:
//  1. collectAdaptec checks the system manufacturer; HP systems delegate to
//     collectHPE.
//  2. hasController reads the controller serial number from sysfs and matches
//     it against arcconf GETCONFIG output to determine the controller ID (cid).
//  3. collect calls parseCtrlCard, collectCtrlPD, and collectCtrlLD.
//  4. associate links physical drives to logical drives by serial number.
package raid

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/zx-cc/baize/internal/collector/smart"
	"github.com/zx-cc/baize/pkg/paths"
	"github.com/zx-cc/baize/pkg/shell"
	"github.com/zx-cc/baize/pkg/utils"
)

const (
	// snFile is the relative sysfs path (under the PCI device root) that
	// contains the controller serial number used for controller matching.
	snFile  = "/host0/scsi_host/host0/serial_number"
	arcconf = "/usr/local/hwtool/tool/arcconf"
)

// adaptecController is an internal working struct for Adaptec RAID collection.
type adaptecController struct {
	ctrl *controller
	cid  string // arcconf controller number string, e.g. "1"
}

// collectField is a generic key→value pointer pair used to dispatch parsed
// text output lines to their destination struct fields.
type collectField struct {
	key   string
	value *string
}

// collectAdaptec is the vendor entry point called by the RAID dispatcher.
// It first checks the system manufacturer via dmidecode; on HP hardware it
// delegates to collectHPE.  Otherwise it runs the Adaptec-specific flow.
func collectAdaptec(i int, c *controller) error {
	ctrl := &adaptecController{
		ctrl: c,
	}

	output, err := shell.Run("dmidecode", "-s", "system-manufacturer")
	if err != nil {
		return err
	}

	if bytes.HasPrefix(bytes.TrimSpace(output), []byte("HP")) {
		return collectHPE(i, c)
	}

	if !ctrl.hasController(i) {
		return fmt.Errorf("adaptec controller %s not found", c.PCIe.Bus)
	}

	return ctrl.collect()
}

// hasController reads the controller serial number from sysfs and searches
// arcconf slots 0..i for a matching entry.  Sets ac.cid on success.
func (ac *adaptecController) hasController(i int) bool {
	sn, err := os.ReadFile(filepath.Join(paths.SysBusPciDevices, ac.ctrl.PCIe.Bus, snFile))
	if err != nil {
		return false
	}

	for j := 0; j < i+1; j++ {
		output, err := shell.Run(arcconf, "GETCONFIG", strconv.Itoa(j), "AD")
		if err != nil {
			continue
		}
		if len(output) > 0 && bytes.Contains(output, bytes.TrimSpace(sn)) {
			ac.cid = strconv.Itoa(j)
			return true
		}
	}

	return false
}

// arcconfRun runs `arcconf GETCONFIG <args...>` and returns the output.
func arcconfRun(args ...string) ([]byte, error) {
	output, err := shell.Run(arcconf+" GETCONFIG", args...)
	if err != nil {
		return nil, err
	}

	return output, nil
}

// collect runs the three main sub-collectors and the association step.
func (ac *adaptecController) collect() error {

	errs := make([]error, 0, 3)

	if err := ac.parseCtrlCard(); err != nil {
		errs = append(errs, err)
	}

	if err := ac.collectCtrlPD(); err != nil {
		errs = append(errs, err)
	}

	if err := ac.collectCtrlLD(); err != nil {
		errs = append(errs, err)
	}

	ac.associate()

	return errors.Join(errs...)
}

// parseCtrlCard fetches the adapter-level config and populates the controller
// struct with status, mode, firmware, cache size, and RAID counts.
func (ac *adaptecController) parseCtrlCard() error {
	data, err := arcconfRun(ac.cid, "AD")
	if err != nil {
		return err
	}

	ctrlFields := []collectField{
		{"Controller Status", &ac.ctrl.ControllerStatus},
		{"Controller Mode", &ac.ctrl.CurrentPersonality},
		{"Controller Model", &ac.ctrl.ProductName},
		{"Installed memory", &ac.ctrl.CacheSize},
		{"BIOS", &ac.ctrl.BiosVersion},
		{"Firmware", &ac.ctrl.FwVersion},
	}

	scanner := utils.NewScanner(bytes.NewReader(data))
	for {
		k, v, isEnd := scanner.ParseLine(":")
		if isEnd {
			break
		}
		if v == "" {
			continue
		}

		if k == "Logical devices/Failed/Degraded" {
			val := strings.SplitN(v, "/", 3)
			if len(val) >= 3 {
				ac.ctrl.NumberOfRaid = val[0]
				ac.ctrl.FailedRaid = val[1]
				ac.ctrl.DegradedRaid = val[2]
			}
		}

		for _, field := range ctrlFields {
			if field.key == k {
				*field.value = v
			}
		}
	}

	return scanner.Err()
}

// collectCtrlPD fetches the physical device config and dispatches each hard
// drive block to parseCtrlPD.
func (ac *adaptecController) collectCtrlPD() error {
	data, err := arcconfRun(ac.cid, "PD")
	if err != nil {
		return err
	}

	pds := bytes.Split(data, []byte("\n\n"))
	errs := make([]error, 0, len(pds))
	for _, pd := range pds {
		if !bytes.Contains(pd, []byte("Device is a Hard drive")) {
			continue
		}
		if err := ac.parseCtrlPD(pd); err != nil {
			errs = append(errs, err)
		}
	}

	return errors.Join(errs...)
}

// parseCtrlPD parses a single PD block from arcconf GETCONFIG PD output,
// populates a physicalDrive struct, and collects SMART data via aacraid.
func (ac *adaptecController) parseCtrlPD(data []byte) error {
	pd := &physicalDrive{}
	pdFields := []collectField{
		{"State", &pd.State},
		{"Block Size", &pd.PhysicalSectorSize},
		{"Transfer Speed", &pd.LinkSpeed},
		{"Vendor", &pd.Vendor},
		{"Model", &pd.ModelName},
		{"Firmware", &pd.FirmwareVersion},
		{"Serial Number", &pd.SN},
		{"World-wide name", &pd.WWN},
		{"Write cache", &pd.WriteCache},
		{"S.M.A.R.T.", &pd.SmartAlert},
	}

	errs := make([]error, 0, 2)

	scanner := utils.NewScanner(bytes.NewReader(data))
	for {
		k, v, isEnd := scanner.ParseLine(":")
		if isEnd {
			break
		}

		if v == "" {
			continue
		}

		if k == "Reported Location" {
			val := strings.Split(v, ",")
			if len(val) >= 2 {
				pd.EnclosureId = strings.Fields(val[0])[1]
				pd.SlotId = strings.Fields(val[1])[1]
				pd.Location = fmt.Sprintf("/c%s/e%s/s%s", ac.cid, pd.EnclosureId, pd.SlotId)
			}
			continue
		}

		for _, field := range pdFields {
			if field.key == k {
				*field.value = v
			}
		}
	}

	if err := scanner.Err(); err != nil {
		errs = append(errs, err)
	}

	cid, _ := strconv.Atoi(ac.cid)
	err := pd.getSMARTData(smart.Option{
		Type:  "aacraid",
		Block: defaultBlock,
		Did:   fmt.Sprintf("%d,%s,%s", cid, pd.EnclosureId, pd.SlotId)})
	if err != nil {
		errs = append(errs, err)
	}

	ac.ctrl.PhysicalDrives = append(ac.ctrl.PhysicalDrives, pd)

	return errors.Join(errs...)
}

// collectCtrlLD fetches the logical device config and dispatches each LD block
// to parseCtrlLD.
func (ac *adaptecController) collectCtrlLD() error {
	data, err := arcconfRun(ac.cid, "LD")
	if err != nil {
		return err
	}

	lds := bytes.Split(data, []byte("\n\n"))
	errs := make([]error, 0, len(lds))

	for _, ld := range lds {
		if !bytes.Contains(ld, []byte("Logical Device number")) {
			continue
		}
		if err := ac.parseCtrlLD(ld); err != nil {
			errs = append(errs, err)
		}
	}

	return errors.Join(errs...)
}

// parseCtrlLD parses a single LD block from arcconf GETCONFIG LD output and
// populates a logicalDrive struct.
func (ac *adaptecController) parseCtrlLD(data []byte) error {
	ld := &logicalDrive{}
	ldFields := []collectField{
		{"Logical Device name", &ld.Location},
		{"RAID Level", &ld.Type},
		{"State of Logical Drive", &ld.State},
		{"Size", &ld.Capacity},
	}

	scanner := utils.NewScanner(bytes.NewReader(data))
	for {
		k, v, isEnd := scanner.ParseLine(":")
		if isEnd {
			break
		}
		if strings.HasPrefix(k, "Logical Device number") {
			parts := strings.Fields(k)
			ld.VD = parts[len(parts)-1]
			continue
		}

		if v == "" {
			continue
		}

		if strings.HasPrefix(k, "Segment ") {
			parts := strings.Fields(v)
			ld.pds = append(ld.pds, parts[len(parts)-1])
			continue
		}

		for _, field := range ldFields {
			if field.key == k {
				*field.value = v
			}
		}
	}

	ac.ctrl.LogicalDrives = append(ac.ctrl.LogicalDrives, ld)

	return scanner.Err()
}

// associate links physical drives to their parent logical drives by matching
// the physical drive serial number against the LD segment list.
func (ac *adaptecController) associate() {
	for _, ld := range ac.ctrl.LogicalDrives {
		for _, disk := range ld.pds {
			for _, pd := range ac.ctrl.PhysicalDrives {
				if disk == pd.SN {
					ld.PhysicalDrives = append(ld.PhysicalDrives, pd)
					break
				}
			}
		}
	}
}
