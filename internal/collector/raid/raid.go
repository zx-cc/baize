// Package raid discovers and collects storage controller and NVMe device
// information from the local system.
//
// Supported RAID controller vendors (identified by PCI vendor ID):
//   - Broadcom / LSI   (0x1000) — via storcli
//   - Microchip/Adaptec (0x9005) — via arcconf
//   - HPE Smart Array  (0x103C) — via hpssacli
//   - Intel VROC       (0x8086) — via mdadm
//
// Direct-attached NVMe drives are discovered by scanning the sysfs PCI device
// tree for class ID 0x0108 (NVMe storage controller) and collecting SMART data
// via smartctl.
//
// For each RAID controller the following sub-information is collected:
//   - Controller card details (firmware, cache, PCIe link)
//   - Physical drives with SMART data
//   - Logical drives (virtual disks) with RAID level and state
//   - Enclosures / backplanes
//   - Battery / CacheVault modules
package raid

import (
	"errors"
	"path/filepath"
	"slices"

	"github.com/zx-cc/baize/internal/collector/pci"
	"github.com/zx-cc/baize/internal/collector/smart"
	"github.com/zx-cc/baize/pkg/paths"
	"github.com/zx-cc/baize/pkg/utils"
)

// vendorID represents a PCI vendor ID string (4-digit hex, uppercase, e.g. "1000").
type vendorID string

// Supported RAID controller vendor PCI IDs.
const (
	VendorLSI     vendorID = "1000" // Broadcom / LSI
	VendorHPE     vendorID = "103C" // Hewlett-Packard Enterprise (Smart Array)
	VendorIntel   vendorID = "8086" // Intel VROC (Virtual RAID on CPU)
	VendorAdaptec vendorID = "9005" // Microchip / Adaptec
)

// ctrlHandler pairs a PCI vendor ID with its vendor-specific collection
// function.  The function receives the total controller count and a pointer to
// the controller struct it should populate.
type ctrlHandler struct {
	id vendorID
	fn func(int, *controller) error
}

// ctrlHandlers is the ordered dispatch table used to route each detected RAID
// controller PCI device to its vendor-specific collector.
var ctrlHandlers = []ctrlHandler{
	{VendorLSI, collectLSI},
	{VendorAdaptec, collectAdaptec},
	{VendorHPE, collectHPE},
	{VendorIntel, collectIntel},
}

// New returns an initialised Controllers collector with pre-allocated slices
// for RAID controllers and NVMe devices.
func New() *Controllers {
	return &Controllers{
		Controller: make([]*controller, 0, 2),
		NVMe:       make([]*nvme, 0, 8),
	}
}

// Collect enumerates all PCI storage devices, then concurrently collects NVMe
// and RAID controller data.  Returns a joined error if any sub-collection fails.
func (c *Controllers) Collect() error {
	allPCI, err := pci.Collect()
	if err != nil {
		return err
	}

	var (
		ctrlPCI []*pci.PCI
		nvmePCI []*pci.PCI
	)
	for _, p := range allPCI {

		switch {
		case slices.Contains([]string{"0104", "0107"}, p.ClassID):
			ctrlPCI = append(ctrlPCI, p)
		case p.ClassID == "0108":
			nvmePCI = append(nvmePCI, p)
		}
	}

	if len(ctrlPCI) == 0 && len(nvmePCI) == 0 {
		return errors.New("NVMe and controller not found")
	}

	getDefaultBlock() // init default block device
	errs := make([]error, 0, 2)

	if len(nvmePCI) > 0 {
		if err := c.collectNVMe(nvmePCI); err != nil {
			errs = append(errs, err)
		}
	}

	if len(ctrlPCI) > 0 {
		if err := c.collectCtrl(ctrlPCI); err != nil {
			errs = append(errs, err)
		}
	}

	return errors.Join(errs...)
}

// collectCtrl iterates the supplied RAID controller PCI devices, matches each
// to its vendor handler, and appends the populated controller to c.Controller.
func (c *Controllers) collectCtrl(ctrlPCI []*pci.PCI) error {
	errs := make([]error, 0, len(ctrlPCI))

	for _, cp := range ctrlPCI {
		ctrl := &controller{
			PCIe: cp,
		}

		for _, hanler := range ctrlHandlers {
			if hanler.id == vendorID(cp.VendorID) {
				if err := hanler.fn(len(ctrlPCI), ctrl); err != nil {
					errs = append(errs, err)
				}
			}
		}

		c.Controller = append(c.Controller, ctrl)
	}

	return errors.Join(errs...)
}

// collectNVMe iterates the supplied NVMe PCI devices, resolves each to a block
// device path via sysfs, collects SMART data, and enumerates namespaces.
func (c *Controllers) collectNVMe(nvmePCI []*pci.PCI) error {
	errs := make([]error, 0, len(nvmePCI))
	for _, np := range nvmePCI {
		n := &nvme{
			PCIe: np,
		}

		nvmePath := filepath.Join(paths.SysBusPciDevices, np.Bus, "nvme")
		names, err := filepath.Glob(filepath.Join(nvmePath, "nvme*"))
		if err != nil {
			c.NVMe = append(c.NVMe, n)
			errs = append(errs, err)
			continue
		}

		if len(names) == 1 {
			n.MappingFile = "/dev/" + filepath.Base(names[0])

			if err := n.getSMARTData(smart.Option{
				Type:  "nvme",
				Block: n.MappingFile,
			}); err != nil {
				errs = append(errs, err)
			}

			namespaces, err := filepath.Glob(filepath.Join(nvmePath, filepath.Base(names[0]), "nvme*"))
			if err != nil {
				errs = append(errs, err)
				c.NVMe = append(c.NVMe, n)
				continue
			}

			for _, ns := range namespaces {
				n.Namespaces = append(n.Namespaces, "/dev/"+filepath.Base(ns))
			}
		}

		c.NVMe = append(c.NVMe, n)
	}

	return errors.Join(errs...)
}

// pdSMART maps a parsed SMART field value to its destination in a physicalDrive.
type pdSMART struct {
	smartField string  // source value from the SMART result
	pdField    *string // destination pointer in the physicalDrive struct
}

// getSMARTData fetches SMART data for the physical drive using the supplied
// Option and merges non-empty fields into the drive struct.
func (pd *physicalDrive) getSMARTData(so smart.Option) error {
	s, err := smart.GetSmartctlData(so)
	if err != nil {
		return err
	}

	fields := []pdSMART{
		{s.Capacity, &pd.Capacity},
		{s.Firmware, &pd.FirmwareVersion},
		{s.FormFactor, &pd.FormFactor},
		{s.MediaType, &pd.MediaType},
		{s.ModelName, &pd.ModelName},
		{s.PN, &pd.PN},
		{s.PowerOn, &pd.PowerOnTime},
		{s.Protocol, &pd.ProtocolType},
		{s.ProtocolVer, &pd.ProtocolVersion},
		{s.ReadCache, &pd.ReadCache},
		{s.Rotation, &pd.RotationRate},
		{s.SN, &pd.SN},
		{s.Temperature, &pd.Temperature},
		{s.Vendor, &pd.Vendor},
		{s.WriteCache, &pd.WriteCache},
	}

	for _, f := range fields {
		if *f.pdField == "" && f.smartField != "" {
			*f.pdField = f.smartField
		}
	}

	pd.SMARTStatus = s.SMARTStatus
	pd.SMARTAttributes = s.SMARTAttrs

	return nil
}

// defaultBlock is the fallback block device path used by controllers that need
// a host block device to route SMART commands (e.g. HPE cciss, Adaptec aacraid).
var defaultBlock = "/dev/sda"

// getDefaultBlock queries sysfs for the first available block device and
// updates defaultBlock so that controller-specific SMART queries have a valid
// host device path to use.
func getDefaultBlock() {
	blocks := utils.GetBlockFromSysfs()
	if len(blocks) == 0 {
		return
	}

	defaultBlock = "/dev/" + blocks[0]
}

// Name returns the module identifier used for routing by the collector manager.
func (c *Controllers) Name() string {
	return "RAID"
}

// Jprintln serialises the collected storage data to JSON and writes it to stdout.
func (c *Controllers) Jprintln() error {
	return utils.JSONPrintln(c)
}

// Sprintln prints a brief RAID/NVMe summary to stdout.
func (c *Controllers) Sprintln() {}

// Lprintln prints a detailed RAID/NVMe report to stdout.
func (c *Controllers) Lprintln() {}
