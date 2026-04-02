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
	procMdstat = "/proc/mdstat"
	mdadm      = "/usr/sbin/mdadm"
)

type intelController struct {
	ctrl []*vroc
	lds  []*logicalDrive
	pds  []*physicalDrive
}

type vroc struct {
	ctrl    *controller
	pds     []string
	pciAddr string
}

var (
	intelCtrls = &intelController{}
	intelOnece sync.Once
)

func collectIntel(i int, c *controller) error {
	return isFoundIntel(c)
}

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

func mdadmRun(args ...string) ([]byte, error) {
	return shell.Run(mdadm, args...)
}

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

func (ic *intelController) parseCtrlPD(pd string) error {

	res := &physicalDrive{
		MappingFile: "/dev/" + pd,
	}

	err := res.getSMARTData(smart.Option{
		Type: "jbod", Block: res.MappingFile})

	ic.pds = append(ic.pds, res)

	return err
}

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
