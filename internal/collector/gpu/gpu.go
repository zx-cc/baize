// Package gpu discovers graphics cards by enumerating DRM device nodes under
// /sys/class/drm and falling back to a full PCI scan (class ID 0x03) when DRM
// is unavailable.
//
// Each discovered card is annotated with its PCIe device metadata and a flag
// indicating whether it is an on-board (integrated / management) GPU based on
// a curated vendor:device ID allow-list.
package gpu

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/zx-cc/baize/internal/collector/pci"
	"github.com/zx-cc/baize/pkg/paths"
	"github.com/zx-cc/baize/pkg/utils"
)

// GPU is the top-level collector that holds all discovered graphics cards.
type GPU struct {
	GraphicsCard []*GraphicsCard `json:"graphics_card,omitzero"`
}

// GraphicsCard represents a single GPU device with its PCIe metadata and an
// on-board classification flag.
type GraphicsCard struct {
	IsOnBoard bool     `json:"is_on_board,omitzero" name:"On Board" color:"trueGreen"` // true when the GPU is integrated / management
	PCIe      *pci.PCI `json:"pcie,omitzero"`                                          // Associated PCIe device information
}

const (
	defaultCap = 9 // initial capacity for the GraphicsCard slice
)

var (
	// errNotFound is returned by Collect when no GPU devices are detected.
	errNotFound = errors.New("GPU device not found")

	// onBoardSet is an allow-list of "vendorID:deviceID" strings that identify
	// known on-board / management GPUs (e.g. Matrox, HiSilicon, ASPEED).
	onBoardSet = map[string]struct{}{
		"102b:0522": {},
		"102b:0533": {},
		"102b:0534": {},
		"102b:0536": {},
		"102b:0538": {},
		"19e5:1711": {},
		"1a03:2000": {},
	}
)

// New returns an initialised GPU collector.
func New() *GPU {
	return &GPU{
		GraphicsCard: make([]*GraphicsCard, 0, defaultCap),
	}
}

// Collect attempts GPU discovery via DRM first; if that fails it falls back to
// a generic PCI class scan. Returns errNotFound if both strategies yield nothing.
func (g *GPU) Collect() error {
	if err := g.collectFromDrm(); err == nil {
		return nil
	}

	if err := g.collectGPUFromPCI(); err == nil {
		return nil
	}

	return errNotFound
}

// Name returns the module identifier used for routing by the collector manager.
func (g *GPU) Name() string {
	return "GPU"
}

// Jprintln serialises the collected GPU data to JSON and writes it to stdout.
func (g *GPU) Jprintln() error {
	return utils.JSONPrintln(g)
}

// Sprintln prints a brief GPU summary to stdout.
func (g *GPU) Sprintln() {}

// Lprintln prints a detailed GPU report to stdout.
func (g *GPU) Lprintln() {}

// collectFromDrm enumerates /sys/class/drm for card* entries, resolves each
// to its PCI bus address, and collects PCIe device metadata.
func (g *GPU) collectFromDrm() error {
	dirEntries, err := os.ReadDir(paths.SysClassDrm)
	if err != nil {
		return fmt.Errorf("read %s: %w", paths.SysClassDrm, err)
	}

	errs := make([]error, 0, len(dirEntries)/2)

	for _, entry := range dirEntries {
		dirName := entry.Name()

		if !strings.HasPrefix(dirName, "card") {
			continue
		}

		if strings.Contains(dirName, "-") {
			continue
		}

		devicePath := filepath.Join(paths.SysClassDrm, dirName, "device")
		if !paths.Exists(devicePath) {
			continue
		}

		pciBus, err := utils.ReadLinkBase(devicePath)
		if err != nil {
			errs = append(errs, err)
			continue
		}

		p, err := pci.GetByBus(pciBus)
		if err != nil {
			errs = append(errs, err)
			g.GraphicsCard = append(g.GraphicsCard, &GraphicsCard{
				PCIe: p,
			})
			continue
		}

		g.GraphicsCard = append(g.GraphicsCard, &GraphicsCard{
			PCIe:      p,
			IsOnBoard: isOnBoard(p.VendorID, p.DeviceID),
		})
	}

	return errors.Join(errs...)
}

// collectGPUFromPCI performs a full PCI scan and collects all devices whose
// PCI class ID is 0x03 (Display controller / VGA compatible controller).
func (g *GPU) collectGPUFromPCI() error {
	allPCI, err := pci.Collect()
	if err != nil {
		return err
	}

	for _, p := range allPCI {
		if p.ClassID == "03" {
			g.GraphicsCard = append(g.GraphicsCard, &GraphicsCard{
				PCIe:      p,
				IsOnBoard: isOnBoard(p.VendorID, p.DeviceID),
			})
		}
	}

	return nil
}

// isOnBoard returns true if the vendorID:deviceID combination matches a known
// on-board / management GPU entry in onBoardSet.
func isOnBoard(vendorID, deviceID string) bool {
	var sb strings.Builder
	sb.Grow(9)
	sb.WriteString(vendorID)
	sb.WriteByte(':')
	sb.WriteString(deviceID)

	_, exists := onBoardSet[sb.String()]

	return exists
}
