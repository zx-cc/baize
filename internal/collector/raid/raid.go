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

// vendorID represents a PCI vendor ID string (4-digit hex, uppercase).
type vendorID string

// Supported RAID controller vendor PCI IDs.
const (
	VendorLSI     vendorID = "1000" // Broadcom / LSI
	VendorHPE     vendorID = "103C" // Hewlett-Packard Enterprise (Smart Array)
	VendorIntel   vendorID = "8086" // Intel VROC (Virtual RAID on CPU)
	VendorAdaptec vendorID = "9005" // Microchip / Adaptec
)

type ctrlHandler struct {
	id vendorID
	fn func(int, *controller) error
}

var ctrlHandlers = []ctrlHandler{
	{VendorLSI, collectLSI},
	{VendorAdaptec, collectAdaptec},
	{VendorHPE, collectHPE},
	{VendorIntel, collectIntel},
}

func New() *Controllers {
	return &Controllers{
		Controller: make([]*controller, 0, 2),
		NVMe:       make([]*nvme, 0, 8),
	}
}

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
		if p.ClassID == "01" {
			switch {
			case slices.Contains([]string{"04", "07"}, p.SubClassID):
				ctrlPCI = append(ctrlPCI, p)
			case p.SubClassID == "08":
				nvmePCI = append(nvmePCI, p)
			}
		}
	}

	if len(ctrlPCI) == 0 && len(nvmePCI) == 0 {
		return errors.New("NVMe and controller not found")
	}

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

type pdSMART struct {
	smartField string
	pdField    *string
}

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

var defaultBlock = "/dev/sda"

func getDefaultBlock() {
	blocks := utils.GetBlockFromSysfs()
	if len(blocks) == 0 {
		return
	}

	defaultBlock = "/dev/" + blocks[0]
}
