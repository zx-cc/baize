// Package raid — intel_vroc.go collects storage controller and drive data from
// Intel VROC (Virtual RAID on CPU) using the mdadm management utility.
//
// Intel VROC presents as one or more Intel PCI devices (VID 0x8086).  All
// controller state is read once via a sync.Once and stored in a package-level
// intelController; subsequent calls simply copy the matching sub-struct into
// the caller-supplied controller.
//
// Discovery flow:
//  1. collectIntel/isFoundIntel is called for each Intel PCI storage device.
//  2. intelOnce ensures collectCtrlCard, collectCtrlPD, and collectCtrlLD run
//     exactly once via mdadm --detail-platform and /proc/mdstat.
//  3. associate wires physical drives to logical drives and logical drives to
//     their parent VROC controller.
package raid

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/zx-cc/baize/internal/collector/smart"
	"github.com/zx-cc/baize/pkg/shell"
	"github.com/zx-cc/baize/pkg/utils"
)

const (
	procMdstat = "/proc/mdstat" // kernel software-RAID state file
	mdadm      = "/usr/sbin/mdadm"
)

// intelController caches all VROC controller, physical drive, and logical drive
// data collected from mdadm in a single pass.
type intelController struct {
	ctrl []*vroc
	lds  []*logicalDrive
	pds  []*physicalDrive
}

// vroc represents a single Intel VROC controller instance discovered via
// `mdadm --detail-platform`.
type vroc struct {
	ctrl    *controller
	pds     []string // raw disk identifiers from --detail-platform output
	pciAddr string   // PCIe bus address (e.g. "0000:3b:00.0")
}

var (
	// intelCtrls is the package-level shared state for all Intel VROC controllers.
	intelCtrls = &intelController{}
	// intelOnece ensures that mdadm collection runs exactly once per process.
	intelOnece sync.Once
)

// collectIntel is the vendor entry point called by the RAID dispatcher.
func collectIntel(i int, c *controller) error {
	return isFoundIntel(c)
}

// isFoundIntel triggers the one-time mdadm collection (via sync.Once) and
// then copies the matching VROC controller data into the caller's controller.
func isFoundIntel(c *controller) error {
	var err error

	intelOnece.Do(func() {
		err = intelCtrls.collect()
		intelCtrls.associate()
	})

	for _, ctr := range intelCtrls.ctrl {
		if ctr.pciAddr == c.PCIe.Bus {
			ctr.ctrl.PCIe = c.PCIe
			*c = *ctr.ctrl
		}
	}

	return err
}

// collect runs all three mdadm sub-collectors: card, physical drives, logical
// drives.  Errors are joined and returned.
func (ic *intelController) collect() error {
	errs := make([]error, 0, 3)
	if err := ic.collectCtrlCard(); err != nil {
		errs = append(errs, err)
	}

	if err := ic.collectCtrlPD(); err != nil {
		errs = append(errs, err)
	}

	if err := ic.collectCtrlLD(); err != nil {
		errs = append(errs, err)
	}

	return errors.Join(errs...)
}

// mdadmRun is a thin wrapper around shell.Run for the mdadm binary.
func mdadmRun(args ...string) ([]byte, error) {
	return shell.Run(mdadm, args...)
}

// collectCtrlCard runs `mdadm --detail-platform` and splits the output into
// per-controller blocks separated by blank lines.
func (ic *intelController) collectCtrlCard() error {
	data, err := mdadmRun("--detail-platform")
	if err != nil {
		return err
	}

	ctrls := bytes.SplitSeq(data, []byte("\n\n"))
	var errs []error

	for ctrl := range ctrls {
		if err := ic.parseCtrlCard(bytes.TrimSpace(ctrl)); err != nil {
			errs = append(errs, err)
		}
	}

	return errors.Join(errs...)
}

// parseCtrlCard parses a single --detail-platform controller block, extracting
// supported RAID levels, maximum drive count, I/O controller PCIe address, and
// associated disk devices (NVMe-under-VMD and Port-attached).
func (ic *intelController) parseCtrlCard(data []byte) error {

	if len(data) == 0 {
		return nil
	}

	res := &vroc{
		ctrl: &controller{},
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

		switch {
		case k == "RAID Levels":
			res.ctrl.RaidLevelSupported = v
		case k == "Max Disks":
			res.ctrl.SupportedDrives = v
		case k == "I/O Controller":
			if res.pciAddr != "" {
				continue
			}
			res.pciAddr = filepath.Base(strings.Fields(v)[0])
		case k == "NVMe under VMD":
			disk := strings.Fields(v)[0]
			res.pds = append(res.pds, disk)
		case strings.HasPrefix(k, "Port"):
			if !strings.Contains(v, "no device attached") {
				disk := strings.Fields(v)[0]
				res.pds = append(res.pds, disk+" "+k)
			}
		}
	}

	ic.ctrl = append(ic.ctrl, res)

	return scanner.Err()
}

// collectCtrlPD enumerates block devices via lsblk and collects SMART data
// for each one using the jbod (direct pass-through) access mode.
func (ic *intelController) collectCtrlPD() error {

	pds := utils.GetBlockByLsblk()
	if len(pds) == 0 {
		return nil
	}

	errs := make([]error, 0, len(pds))
	for _, pd := range pds {
		if err := ic.parseCtrlPD(pd); err != nil {
			errs = append(errs, err)
		}
	}

	return errors.Join(errs...)
}

// parseCtrlPD collects SMART data for a single block device name.
func (ic *intelController) parseCtrlPD(pd string) error {

	res := &physicalDrive{
		MappingFile: "/dev/" + pd,
	}

	err := res.getSMARTData(smart.Option{
		Type: "jbod", Block: res.MappingFile})

	ic.pds = append(ic.pds, res)

	return err
}

// collectCtrlLD parses /proc/mdstat for active md arrays and calls parseCtrlLD
// for each one.
func (ic *intelController) collectCtrlLD() error {

	file, err := os.Open(procMdstat)
	if err != nil {
		return err
	}
	defer file.Close()

	var errs []error
	scanner := utils.NewScanner(file)
	for {
		k, v, isEnd := scanner.ParseLine(":")
		if isEnd {
			break
		}
		if v == "" || !strings.HasPrefix(v, "active") {
			continue
		}

		if err := ic.parseCtrlLD(k); err != nil {
			errs = append(errs, err)
		}
	}

	return errors.Join(errs...)
}

// parseCtrlLD runs `mdadm --detail /dev/<md>` and populates a logicalDrive
// struct with RAID level, capacity, state, and member drive paths.
func (ic *intelController) parseCtrlLD(md string) error {

	ld := &logicalDrive{
		MappingFile: "/dev/" + md,
	}

	data, err := mdadmRun("--detail", ld.MappingFile)
	if err != nil {
		return err
	}

	fields := []collectField{
		{"Raid Level", &ld.Type},
		{"Array Size", &ld.Capacity},
		{"Total Devices", &ld.NumberOfDrives},
		{"State", &ld.State},
		{"Consistency Policy", &ld.Cache},
		{"UUID", &ld.ScsiNaaId},
	}

	scanner := utils.NewScanner(bytes.NewReader(data))
	for {
		k, v, isEnd := scanner.ParseLine(":")
		if isEnd {
			break
		}

		if v == "" && strings.Contains(k, "/dev/") {
			idx := strings.IndexByte(k, '/')
			ld.pds = append(ld.pds, k[idx:])
		}

		for _, f := range fields {
			if f.key == k {
				if k == "Array Size" {
					v = strings.Fields(v)[0]
				}
				*f.value = v
				break
			}
		}
	}

	ic.lds = append(ic.lds, ld)

	return scanner.Err()
}

// associate wires physical drives to their parent VROC controller and
// links logical drives to both their member physical drives and their
// parent VROC controller.
func (ic *intelController) associate() {
	if len(ic.ctrl) == 0 || len(ic.pds) == 0 || len(ic.lds) == 0 {
		return
	}

	for _, ctr := range ic.ctrl {
		if len(ctr.pds) == 0 {
			continue
		}
		for _, disk := range ctr.pds {
			parts := strings.Fields(disk)
			for _, pd := range ic.pds {
				if pd.MappingFile == parts[0] {
					if len(parts) > 1 {
						pd.Location = parts[1]
					}
					ctr.ctrl.PhysicalDrives = append(ctr.ctrl.PhysicalDrives, pd)
				}
			}
		}
	}

	for _, ld := range ic.lds {
		if len(ld.pds) == 0 {
			continue
		}

		for _, ctr := range ic.ctrl {
			for _, pd := range ctr.ctrl.PhysicalDrives {
				for _, disk := range ld.pds {
					if pd.MappingFile == disk {
						ld.PhysicalDrives = append(ld.PhysicalDrives, pd)
					}
				}
			}
			ctr.ctrl.LogicalDrives = append(ctr.ctrl.LogicalDrives, ld)
		}
	}
}
