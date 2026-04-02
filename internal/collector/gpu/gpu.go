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

type GPU struct {
	GraphicsCard []*GraphicsCard `json:"graphics_card,omitzero"`
}

type GraphicsCard struct {
	IsOnBoard bool     `json:"is_on_board,omitzero" name:"On Board" color:"trueGreen"`
	PCIe      *pci.PCI `json:"pcie,omitzero"`
}

const (
	defaultCap     = 9
	maxConcurrency = 4
)

var (
	errNotFound = errors.New("GPU device not found")

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

func New() *GPU {
	return &GPU{
		GraphicsCard: make([]*GraphicsCard, 0, defaultCap),
	}
}

func (g *GPU) Collect() error {
	if err := g.collectFromDrm(); err == nil {
		return nil
	}

	if err := g.collectGPUFromPCI(); err == nil {
		return nil
	}

	return errNotFound
}

func (g *GPU) Name() string {
	return "cpu"
}

// func (g *GPU) JSON() error {
// 	return utils.JSONPrintln(g)
// }

// func (g *GPU) DetailPrintln() {
// 	utils.PrinterInstance.Print(g, "detail")
// }

// // BriefPrintln prints a concise GPU summary to stdout.
// func (c *GPU) BriefPrintln() {
// 	utils.PrinterInstance.Print(c, "brief")
// }

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

func isOnBoard(vendorID, deviceID string) bool {
	var sb strings.Builder
	sb.Grow(9)
	sb.WriteString(vendorID)
	sb.WriteByte(':')
	sb.WriteString(deviceID)

	_, exists := onBoardSet[sb.String()]

	return exists
}
