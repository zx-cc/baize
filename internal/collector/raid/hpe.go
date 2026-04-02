package raid

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"sync/atomic"

	"github.com/zx-cc/baize/internal/collector/smart"
	"github.com/zx-cc/baize/pkg/shell"
	"github.com/zx-cc/baize/pkg/utils"
)

const (
	hpssacli = "/usr/local/beidou/tool/hpssacli"
)

type hpeController struct {
	ctrl      *controller
	cid       string
	failedPDs uint32
}

var (
	hpePDRegex        = regexp.MustCompile(`physicaldrive (\d+I:\d+:\d+) \(port.*?\)`)
	hpeLDRegex        = regexp.MustCompile(`logicaldrive (\d+) \(.*?\)`)
	hpeEnclosureRegex = regexp.MustCompile(`Internal Drive Cage at Port (\d+I), Box (\d+), ([A-Za-z]+)`)
)

func collectHPE(i int, c *controller) error {
	hpeCtr := &hpeController{
		ctrl: c,
	}

	if !hpeCtr.hasController(i) {
		return fmt.Errorf("hpe controller %s not found", c.PCIe.Bus)
	}

	err := hpeCtr.collect()

	hpeCtr.associate()

	return err
}

func (h *hpeController) hasController(num int) bool {
	for i := 0; i < num; i++ {
		script := fmt.Sprintf("%s ctrl slot=%d show | grep -i %s", hpssacli, i, h.ctrl.PCIe.Bus)
		stdout, err := shell.RunShell(script)
		if err == nil && len(stdout) > 0 {
			h.cid = strconv.Itoa(i)
			return true
		}
	}

	return false
}

func hpssacliRun(args ...string) ([]byte, error) {
	return shell.Run(hpssacli, args...)
}

func (h *hpeController) collect() error {
	data, err := hpssacliRun("ctrl", "slot="+h.cid, "show", "config")
	if err != nil {
		return err
	}

	errs := make([]error, 0, 4)
	if err := h.parseCtrlCard(); err != nil {
		errs = append(errs, err)
	}

	pds := hpePDRegex.FindAllSubmatch(data, -1)
	if err := h.collectCtrlPD(pds); err != nil {
		errs = append(errs, err)
	}

	lds := hpeLDRegex.FindAllSubmatch(data, -1)
	if err := h.collectCtrlLD(lds); err != nil {
		errs = append(errs, err)
	}

	els := hpeEnclosureRegex.FindAllSubmatch(data, -1)
	if err := h.collectCtrlEnclosure(els); err != nil {
		errs = append(errs, err)
	}

	return errors.Join(errs...)
}

func (h *hpeController) parseCtrlCard() error {

	data, err := hpssacliRun("ctrl", "slot="+h.cid, "show")
	if err != nil {
		return fmt.Errorf("controller %s: %w", h.cid, err)
	}

	cardFields := []collectField{
		{"Controller Status", &h.ctrl.ControllerStatus},
		{"Controller Mode", &h.ctrl.CurrentPersonality},
		{"Firmware Version", &h.ctrl.Firmware},
		{"Total Cache Size", &h.ctrl.CacheSize},
		{"Interface", &h.ctrl.HostInterface},
		{"Serial Number", &h.ctrl.SerialNumber},
	}

	scanner := utils.NewScanner(bytes.NewReader(data))
	for {
		key, value, isEnd := scanner.ParseLine(":")
		if isEnd {
			break
		}

		if strings.HasPrefix(key, "Smart ") || strings.HasPrefix(key, "HPE Smart Array") {
			h.ctrl.ProductName = key
			continue
		}

		if key == "Battery/Capacitor Status" {
			h.ctrl.Battery = append(h.ctrl.Battery, &battery{State: value})
		}

		for _, f := range cardFields {
			if key == f.key {
				*f.value = value
			}
		}
	}

	return scanner.Err()
}

func (h *hpeController) collectCtrlPD(pds [][][]byte) error {
	if len(pds) == 0 {
		return nil
	}

	errs := make([]error, 0, len(pds))
	h.ctrl.PhysicalDrives = make([]*physicalDrive, 0, len(pds)/2)
	for _, pd := range pds {
		if err := h.parseCtrlPD(pd[1]); err != nil {
			errs = append(errs, fmt.Errorf("parse %s pd: %w", string(pd[1]), err))
			continue
		}
	}

	return errors.Join(errs...)
}

func (h *hpeController) parseCtrlPD(p []byte) error {
	data, err := hpssacliRun("ctrl", "slot="+h.cid, "pd", string(p), "show")
	if err != nil {
		return err
	}

	pd := &physicalDrive{
		Location: string(p),
	}

	fieldFunc := map[string]func(string){
		"Port": func(v string) { pd.EnclosureId = v },
		"Box":  func(v string) { pd.EnclosureId += ":" + v },
		"Bay": func(v string) {
			pd.SlotId = v
			did, _ := strconv.Atoi(v)
			pd.DeviceId = strconv.Itoa(did - 1)
		},
		"Status":                  func(v string) { pd.State = v },
		"Interface Type":          func(v string) { pd.ProtocolType = v },
		"Size":                    func(v string) { pd.Capacity = v },
		"Firmware Revision":       func(v string) { pd.FirmwareVersion = v },
		"Serial Number":           func(v string) { pd.SN = v },
		"WWID":                    func(v string) { pd.WWN = v },
		"Model":                   func(v string) { pd.ModelName = v },
		"Current Temperature (C)": func(v string) { pd.Temperature = v + " ℃" },
		"PHY Transfer Rate":       func(v string) { pd.DeviceSpeed = v },
		"Logical/Physical Block Size": func(v string) {
			if parts := strings.Split(v, "/"); len(parts) == 2 {
				pd.LogicalSectorSize = parts[0] + " B"
				pd.PhysicalSectorSize = parts[1] + " kB"
			}
		},
	}

	var errs []error
	scanner := utils.NewScanner(bytes.NewReader(data))
	for {
		key, value, isEnd := scanner.ParseLine(":")
		if isEnd {
			break
		}

		if handler, ok := fieldFunc[key]; ok {
			handler(value)
		}
	}

	if err := scanner.Err(); err != nil {
		errs = append(errs, err)
	}

	if err := h.hpeSMART(pd); err != nil {
		errs = append(errs, err)
	}

	h.ctrl.PhysicalDrives = append(h.ctrl.PhysicalDrives, pd)

	return errors.Join(errs...)
}

func (h *hpeController) hpeSMART(pd *physicalDrive) error {
	if pd.State == "Failed" {
		atomic.AddUint32(&h.failedPDs, 1)
		return nil
	}

	if block := utils.GetBlockByWWN(pd.WWN); block != "" {
		pd.MappingFile = block
		if err := pd.getSMARTData(smart.Option{
			Type: "jbod", Block: block}); err != nil {
			return err
		}
		return nil
	}

	if did, err := strconv.Atoi(pd.DeviceId); err == nil {
		useID := did - int(atomic.LoadUint32(&h.failedPDs))
		err := pd.getSMARTData(smart.Option{
			Type:  "cciss",
			Block: defaultBlock,
			Did:   strconv.Itoa(useID)})
		if err != nil {
			return err
		}
	}

	return nil
}

func (h *hpeController) collectCtrlLD(lds [][][]byte) error {
	if len(lds) == 0 {
		return nil
	}

	errs := make([]error, 0, len(lds))
	h.ctrl.LogicalDrives = make([]*logicalDrive, 0, len(lds)/2)

	for _, ld := range lds {
		if err := h.parseCtrlLD(ld[1]); err != nil {
			errs = append(errs, fmt.Errorf("parse ld %s: %w", string(ld[1]), err))
		}
	}

	return errors.Join(errs...)
}

func (h *hpeController) parseCtrlLD(ld []byte) error {
	data, err := hpssacliRun("ctrl", "slot="+h.cid, "ld", string(ld), "show")
	if err != nil {
		return err
	}

	res := &logicalDrive{
		Location: fmt.Sprintf("/c%s/v%s", h.cid, ld),
	}

	fieldsMap := map[string]func(string){
		"Size":              func(v string) { res.Capacity = v },
		"Fault Tolerance":   func(v string) { res.Type = "RAID " + v },
		"Strip Size":        func(v string) { res.StripSize = v },
		"Status":            func(v string) { res.State = v },
		"Caching":           func(v string) { res.Cache = v },
		"Unique Identifier": func(v string) { res.ScsiNaaId = v },
		"Disk Name":         func(v string) { res.MappingFile = v },
	}

	var array string
	scanner := utils.NewScanner(bytes.NewReader(data))
	for {
		key, value, isEnd := scanner.ParseLine(":")
		if isEnd {
			break
		}

		if strings.HasPrefix(key, "physicaldrive") {
			parts := strings.Fields(key)
			res.pds = append(res.pds, parts[1])
			continue
		}

		if strings.HasPrefix(key, "Array") {
			array = key
			continue
		}

		if fn, exists := fieldsMap[key]; exists {
			fn(value)
		}
	}

	var errs []error

	if err := scanner.Err(); err != nil {
		errs = append(errs, err)
	}

	if len(res.pds) == 0 && array != "" {
		if err := parseArrayPD(res, h.cid, array); err != nil {
			errs = append(errs, err)
		}
	}

	h.ctrl.LogicalDrives = append(h.ctrl.LogicalDrives, res)

	return errors.Join(errs...)
}

func parseArrayPD(res *logicalDrive, cid, array string) error {
	data, err := hpssacliRun("ctrl", fmt.Sprintf("slot=%s", cid), array, "pd", "all", "show")
	if err != nil {
		return fmt.Errorf("%s : %w", array, err)
	}

	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "physicaldrive") {
			parts := strings.Fields(line)
			res.pds = append(res.pds, parts[1])
		}
	}

	return scanner.Err()
}

func (h *hpeController) collectCtrlEnclosure(els [][][]byte) error {
	if len(els) == 0 {
		return nil
	}

	errs := make([]error, 0, len(els))
	for _, el := range els {

		if err := h.parseCtrlEnclosure(el); err != nil {
			errs = append(errs, fmt.Errorf("parse enclosure %s: %w", string(el[1]), err))
		}
	}
	return errors.Join(errs...)
}

func (h *hpeController) parseCtrlEnclosure(el [][]byte) error {
	if len(el) != 4 {
		return fmt.Errorf("enclosure match error:%v", el[1:])
	}

	res := &enclosure{
		Location: fmt.Sprintf("%s:%s", el[1], el[2]),
		State:    string(el[3]),
	}

	data, err := hpssacliRun("ctrl", fmt.Sprintf("slot=%s", h.cid), "enclosure", res.Location, "show")
	if err != nil {
		return fmt.Errorf("enclosure %s: %w", res.Location, err)
	}

	elFields := []collectField{
		{"Drive Bays", &res.PhysicalDriveCount},
		{"Port", &res.ID},
		{"Box", &res.Slots},
		{"Location", &res.EnclosureType},
	}

	scanner := utils.NewScanner(bytes.NewReader(data))
	for {
		key, value, isEnd := scanner.ParseLine(":")
		if isEnd {
			break
		}

		for _, f := range elFields {
			if f.key == key {
				*f.value = value
			}
		}
	}

	h.ctrl.Backplanes = append(h.ctrl.Backplanes, res)
	return scanner.Err()
}

func (h *hpeController) associate() {
	for _, ld := range h.ctrl.LogicalDrives {
		for _, disk := range ld.pds {
			for _, pd := range h.ctrl.PhysicalDrives {
				if disk == pd.Location {
					ld.PhysicalDrives = append(ld.PhysicalDrives, pd)
					break
				}
			}
		}
	}
}
