// Package raid — broadcom_lsi.go collects storage controller and drive data
// from Broadcom/LSI MegaRAID controllers using the storcli management utility.
//
// Discovery flow:
//  1. collectLSI is called with the total RAID controller count and a
//     pre-populated PCI device descriptor.
//  2. isFound scans storcli controller slots (0..n-1) to match the PCIe bus
//     address and determine the storcli controller ID (cid).
//  3. collect fetches the full controller JSON (storcli /cN show J) and
//     dispatches to sub-collectors for card details, physical drives, logical
//     drives, enclosures, and battery/CacheVault units.
//  4. associate links physical drives to their parent logical drives via the
//     drive-group (DG) identifier.
package raid

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/zx-cc/baize/internal/collector/smart"
	"github.com/zx-cc/baize/pkg/shell"
	"github.com/zx-cc/baize/pkg/utils"
)

const (
	storcli = "/usr/local/bin/storcli" // path to the storcli binary
)

// lsiController is an internal working struct that pairs a controller
// descriptor with its storcli controller ID string.
type lsiController struct {
	ctrl *controller
	cid  string // storcli controller index, e.g. "0", "1"
}

// collectLSI is the vendor entry point called by the RAID dispatcher.
// It locates the storcli controller index for the given PCI device,
// runs the full collection, and associates drives with logical drives.
func collectLSI(i int, c *controller) error {

	lsiCtr := &lsiController{
		ctrl: c,
	}

	if !lsiCtr.isFound(i) {
		return fmt.Errorf("lsi controller %s not found", c.PCIe.Bus)
	}

	err := lsiCtr.collect()
	lsiCtr.associate()

	return err
}

// isFound iterates storcli controller slots (0 to i) and checks if any slot
// output contains the PCIe bus address of the controller.  Sets lc.cid on
// success and returns true.
func (lc *lsiController) isFound(i int) bool {
	pcieAddr := lc.ctrl.PCIe.Bus[2 : len(lc.ctrl.PCIe.Bus)-3]
	for j := 0; j < i+1; j++ {
		stdout, err := shell.Run(storcli, "/c"+strconv.Itoa(j), "show")
		if err != nil {
			continue
		}

		if len(stdout) > 0 && bytes.Contains(stdout, []byte(pcieAddr)) {
			lc.cid = strconv.Itoa(j)
			return true
		}
	}

	return false
}

// storcliRun is a thin wrapper around shell.Run for the storcli binary.
func storcliRun(args ...string) ([]byte, error) {
	return shell.Run(storcli, args...)
}

// collect fetches the full JSON output of `storcli /cN show J`, unmarshals it,
// and dispatches to the five sub-collectors.
func (lc *lsiController) collect() error {

	data, err := storcliRun("/c"+lc.cid, "show", "J")
	if err != nil {
		return err
	}

	var js showJSON
	if err := json.Unmarshal(data, &js); err != nil {
		return fmt.Errorf("unmarshal %s show to json: %w", lc.cid, err)
	}

	errs := make([]error, 0, 5)
	if err := lc.parseCtrlCard(); err != nil {
		errs = append(errs, err)
	}

	res := js.Controllers[0].ResponseData
	if err := lc.collectCtrlPD(res.PDList); err != nil {
		errs = append(errs, err)
	}

	if err := lc.collectCtrlLD(res.VDList); err != nil {
		errs = append(errs, err)
	}

	if err := lc.collectCtrlEnclosure(res.EnclosureList); err != nil {
		errs = append(errs, err)
	}

	if err := lc.collectCtrlBattery(res.CacheVaultInfo); err != nil {
		errs = append(errs, err)
	}

	return errors.Join(errs...)
}

// parseCtrlCard runs `storcli /cN show all J` and populates the controller
// struct with firmware versions, hardware config, status, and capabilities.
func (lc *lsiController) parseCtrlCard() error {

	data, err := storcliRun("/c"+lc.cid, "show", "all", "J")
	if err != nil {
		return err
	}

	var js showAllJSON
	if err := json.Unmarshal(data, &js); err != nil {
		return fmt.Errorf("unmarshal %s show all to json: %w", lc.cid, err)
	}

	res := js.Controllers[0].ResponseData

	if b := res.Basics; b != nil {
		lc.ctrl.ProductName = b.Model
		lc.ctrl.SerialNumber = b.SN
		lc.ctrl.ControllerTime = b.CTD
		lc.ctrl.SasAddress = b.SAS
	}

	if v := res.Version; v != nil {
		lc.ctrl.BiosVersion = v.BiosVersion
		lc.ctrl.FwVersion = v.FirmwareVer
		lc.ctrl.Firmware = v.FirmwarePackge
	}

	if b := res.Bus; b != nil {
		lc.ctrl.HostInterface = b.HostInterface
		lc.ctrl.DeviceInterface = b.DeviceInterface
	}

	if s := res.Status; s != nil {
		lc.ctrl.ControllerStatus = s.ControllerStatus
		lc.ctrl.MemoryCorrectableErrors = strconv.Itoa(s.MemoryCeErr)
		lc.ctrl.MemoryUncorrectableErrors = strconv.Itoa(s.MemoryUeErr)
	}

	if a := res.Adapter; a != nil {
		lc.ctrl.SupportedJBOD = a.SupportJBOD
		lc.ctrl.ForeignConfigImport = a.ForeignConfigImport
	}

	if h := res.HwCfg; h != nil {
		lc.ctrl.ChipRevision = h.ChipRevision
		lc.ctrl.FrontEndPortCount = strconv.Itoa(h.FrontEndPortCount)
		lc.ctrl.BackendPortCount = strconv.Itoa(h.BackendPortCount)
		lc.ctrl.NVRAMSize = h.NVRAMSize
		lc.ctrl.FlashSize = h.FlashSize
		lc.ctrl.CacheSize = h.OnBoardMemorySize
	}

	if c := res.Capabilities; c != nil {
		lc.ctrl.SupportedDrives = c.SupportedDrives
		lc.ctrl.RaidLevelSupported = c.RaidLevelSupported
		lc.ctrl.EnableJBOD = c.EnableJBOD
	}

	return nil
}

// collectCtrlPD iterates the physical drive list and calls parseCtrlPD for
// each entry, collecting SMART data in the process.
func (lc *lsiController) collectCtrlPD(pds []*pdList) error {

	if len(pds) == 0 {
		return nil
	}

	if lc.ctrl.PhysicalDrives == nil {
		lc.ctrl.PhysicalDrives = make([]*physicalDrive, 0, len(pds))
	}

	errs := make([]error, 0, len(pds))

	for _, pd := range pds {
		if err := lc.parseCtrlPD(pd); err != nil {
			errs = append(errs, err)
		}
	}

	return errors.Join(errs...)
}

// parseCtrlPD populates a physicalDrive from a storcli PD list entry, fetches
// additional per-drive attributes via `storcli <location> show all`, and
// collects SMART data using the megaraid access mode.
func (lc *lsiController) parseCtrlPD(pd *pdList) error {

	res := &physicalDrive{
		DeviceId:           strconv.Itoa(pd.DID),
		State:              pd.State,
		Capacity:           pd.Size,
		MediaType:          pd.Med,
		ProtocolType:       pd.Intf,
		ModelName:          pd.Model,
		PhysicalSectorSize: pd.SeSz,
		DG:                 parseDG(pd.DG),
	}

	eid, sid, found := strings.Cut(pd.EIDSlt, ":")
	if !found {
		return fmt.Errorf("unexcepted EIDSlt: %s", pd.EIDSlt)
	}
	res.EnclosureId = eid
	res.SlotId = sid
	res.Location = "/c" + lc.cid + "/e" + res.EnclosureId + "/s" + res.SlotId

	data, err := storcliRun(res.Location, "show", "all")
	if err != nil {
		return err
	}

	pdFields := []collectField{
		{"Shield Counter", &res.ShieldCounter},
		{"Media Error Count", &res.MediaErrorCount},
		{"Other Error Count", &res.OtherErrorCount},
		{"Predictive Failure Count", &res.PredictiveFailureCount},
		{"Drive Temperature", &res.Temperature},
		{"S.M.A.R.T alert flagged by drive", &res.SmartAlert},
		{"SN", &res.SN},
		{"WWN", &res.WWN},
		{"Firmware Revision", &res.FirmwareVersion},
		{"Device Speed", &res.DeviceSpeed},
		{"Link Speed", &res.LinkSpeed},
		{"Logical Sector Size", &res.LogicalSectorSize},
		{"Physical Sector Size", &res.PhysicalSectorSize},
	}

	scanner := utils.NewScanner(bytes.NewReader(data))
	for {
		k, v, hasMore := scanner.ParseLine("=")
		if !hasMore {
			break
		}
		if v == "" {
			continue
		}

		for _, f := range pdFields {
			if f.key == k {
				*f.value = v
			}
		}
	}

	errs := make([]error, 0, 2)
	if err := scanner.Err(); err != nil {
		errs = append(errs, err)
	}

	err = res.getSMARTData(smart.Option{
		Type:   "megaraid",
		Did:    res.DeviceId,
		CtrlID: lc.cid,
	})
	if err != nil {
		errs = append(errs, err)
	}

	lc.ctrl.PhysicalDrives = append(lc.ctrl.PhysicalDrives, res)

	return errors.Join(errs...)
}

// parseDG converts the storcli DG field (which may arrive as a string, float,
// or integer depending on the storcli version) to a normalised string.
func parseDG(dg any) string {
	switch v := dg.(type) {
	case string:
		return v
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64)
	case int:
		return strconv.Itoa(v)
	case int64:
		return strconv.FormatInt(v, 10)
	default:
		return "Unknown"
	}
}

// collectCtrlLD iterates the virtual drive list and calls parseCtrlLD for each.
func (lc *lsiController) collectCtrlLD(vds []*vdList) error {

	if len(vds) == 0 {
		return nil
	}

	if lc.ctrl.LogicalDrives == nil {
		lc.ctrl.LogicalDrives = make([]*logicalDrive, 0, len(vds))
	}

	errs := make([]error, 0, len(vds))

	for _, vd := range vds {
		if err := lc.parseCtrlLD(vd); err != nil {
			errs = append(errs, err)
		}
	}

	return errors.Join(errs...)
}

// parseCtrlLD populates a logicalDrive from a storcli VD list entry and
// fetches additional attributes via `storcli <location> show all`.
func (lc *lsiController) parseCtrlLD(vd *vdList) error {

	ld := &logicalDrive{
		Type:     vd.Level,
		State:    vd.State,
		Capacity: vd.Size,
		Consist:  vd.Consist,
		Access:   vd.Access,
		Cache:    vd.Cache,
	}

	dg, vid, found := strings.Cut(vd.DGVD, "/")
	if !found {
		return fmt.Errorf("unexcepted DG/VD: %s", vd.DGVD)
	}
	ld.DG = dg
	ld.VD = vid
	ld.Location = "/c" + lc.cid + "/v" + vid

	data, err := storcliRun(ld.Location, "show", "all")
	if err != nil {
		return err
	}

	fields := []collectField{
		{"Strip Size", &ld.StripSize},
		{"Number of Blocks", &ld.NumberOfBlocks},
		{"Number of Drives Per Span", &ld.NumberOfDrivesPerSpan},
		{"OS Drive Name", &ld.MappingFile},
		{"Creation Date", &ld.CreateTime},
		{"SCSI NAA Id", &ld.ScsiNaaId},
	}
	scanner := utils.NewScanner(bytes.NewReader(data))
	for {
		k, v, hasMore := scanner.ParseLine("=")
		if !hasMore {
			break
		}
		// if v == "" && pdRegexp.MatchString(k) {
		// 	parts := strings.Fields(k)
		// 	pd := "/c" + lc.cid + "/e" + strings.ReplaceAll(parts[0], ":", "/s")
		// 	ld.pds = append(ld.pds, pd)
		// 	continue
		// }

		if v == "" {
			continue
		}

		for _, f := range fields {
			if f.key == k {
				*f.value = v
			}
		}
	}

	lc.ctrl.LogicalDrives = append(lc.ctrl.LogicalDrives, ld)

	return scanner.Err()
}

// collectCtrlEnclosure iterates the enclosure list and calls parseCtrlEnclosure
// for each entry.
func (lc *lsiController) collectCtrlEnclosure(ens []*enclosureList) error {

	if len(ens) == 0 {
		return nil
	}

	if lc.ctrl.Backplanes == nil {
		lc.ctrl.Backplanes = make([]*enclosure, 0, len(ens))
	}

	errs := make([]error, 0, len(ens))

	for _, en := range ens {
		if err := lc.parseCtrlEnclosure(en); err != nil {
			errs = append(errs, err)
		}
	}

	return errors.Join(errs...)
}

// parseCtrlEnclosure populates an enclosure struct and fetches additional
// attributes via `storcli <location> show all`.
func (lc *lsiController) parseCtrlEnclosure(en *enclosureList) error {

	enl := &enclosure{
		ID:                 strconv.Itoa(en.EID),
		State:              en.State,
		Slots:              strconv.Itoa(en.Slots),
		Location:           fmt.Sprintf("/c%s/e%d", lc.cid, en.EID),
		PhysicalDriveCount: strconv.Itoa(en.PD),
	}

	data, err := storcliRun(enl.Location, "show", "all")
	if err != nil {
		return err
	}

	fields := []collectField{
		{"Connector Name", &enl.ConnectorName},
		{"Enclosure Type", &enl.EnclosureType},
		{"Enclosure Serial Number", &enl.EnclosureSerialNumber},
		{"Device Type", &enl.DeviceType},
		{"Vendor Identification", &enl.Vendor},
		{"Product Identification", &enl.ProductIdentification},
		{"Product Revision Level", &enl.ProductRevisionLevel},
	}

	scanner := utils.NewScanner(bytes.NewReader(data))
	for {
		k, v, hasMore := scanner.ParseLine("=")
		if !hasMore {
			break
		}
		if v == "" {
			continue
		}

		for _, f := range fields {
			if f.key == k {
				*f.value = v
			}
		}
	}

	lc.ctrl.Backplanes = append(lc.ctrl.Backplanes, enl)

	return scanner.Err()
}

// collectCtrlBattery converts storcli CacheVault entries to battery structs.
func (lc *lsiController) collectCtrlBattery(bbus []*cacheVaultInfo) error {

	if len(bbus) == 0 {
		return nil
	}

	if lc.ctrl.Battery == nil {
		lc.ctrl.Battery = make([]*battery, 0, len(bbus))
	}

	for _, bbu := range bbus {
		cachevault := &battery{
			Model:         bbu.Model,
			State:         bbu.State,
			Temperature:   bbu.Temp,
			RetentionTime: bbu.RetentionTime,
			Mode:          bbu.Mode,
			MfgDate:       bbu.MfgDate,
		}

		lc.ctrl.Battery = append(lc.ctrl.Battery, cachevault)
	}

	return nil
}

// associate links each physical drive to its parent logical drive by matching
// the drive-group (DG) identifier.
func (lc *lsiController) associate() {
	// for _, ld := range lc.ctrl.LogicalDrives {
	// 	for _, disk := range ld.pds {
	// 		for _, pd := range lc.ctrl.PhysicalDrives {
	// 			if disk == pd.Location {
	// 				ld.PhysicalDrives = append(ld.PhysicalDrives, pd)
	// 				break
	// 			}
	// 		}
	// 	}
	// }

	for _, ld := range lc.ctrl.LogicalDrives {
		for _, pd := range lc.ctrl.PhysicalDrives {
			if ld.DG == pd.DG {
				ld.PhysicalDrives = append(ld.PhysicalDrives, pd)
			}
		}
	}
}
