// Package network — nic.go collects physical NIC details including PCI device
// metadata, ethtool ring-buffer / channel configuration, and LLDP neighbour
// information reported by lldpctl.
package network

import (
	"bufio"
	"bytes"
	"errors"
	"strings"

	"github.com/zx-cc/baize/internal/collector/pci"
	"github.com/zx-cc/baize/pkg/shell"
	"github.com/zx-cc/baize/pkg/utils"
)

// collectPhyFromPCI resolves the PCI bus address for the NIC, populates PCIe
// device metadata, and runs the ethtool / lldpctl sub-collectors.
func (p *PhyInterface) collectPhyFromPCI(bus string) error {
	errs := make([]error, 0, 4)
	pciInfo, err := pci.GetByBus(bus)
	if err != nil {
		errs = append(errs, err)
	}

	p.PCI = pciInfo

	if err := p.collectEthtoolChannel(); err != nil {
		errs = append(errs, err)
	}

	if err := p.collectEthtoolRingBuffer(); err != nil {
		errs = append(errs, err)
	}

	if err := p.collectLLDPNeighbors(); err != nil {
		errs = append(errs, err)
	}

	return errors.Join(errs...)
}

// LLDP keyvalue field name constants used when parsing `lldpctl -f keyvalue`
// output for a specific interface.
const (
	fieldChassisMac      = "chassis.mac"
	fieldChassisName     = "chassis.name"
	fieldChassisMgmtIP   = "chassis.mgmt-ip"
	fieldPortIfname      = "port.ifname"
	fieldPortAggregation = "port.aggregation"
	fieldVlanID          = "vlan.vlan-id"
	fieldVlanPvid        = "vlan.pvid"
	fieldPpvidSupport    = "ppvid.support"
	fieldPpvidEnabled    = "ppvid.enabled"
)

// collectLLDPNeighbors runs `lldpctl <iface> -f keyvalue` and parses the
// key=value output to populate the LLDP neighbour information for the NIC.
// The lldp.<iface>. prefix is stripped from each key before dispatching.
func (p *PhyInterface) collectLLDPNeighbors() error {
	data, err := shell.Run("lldpctl", p.DeviceName, "-f", "keyvalue")
	if err != nil {
		return err
	}

	var prefixBuilder strings.Builder
	prefixBuilder.Grow(7 + len(nic))
	prefixBuilder.WriteString("lldp.")
	prefixBuilder.WriteString(p.DeviceName)
	prefixBuilder.WriteByte('.')
	prefix := prefixBuilder.String()
	prefixLen := len(prefix)

	scanner := utils.NewScanner(bytes.NewReader(data))
	for {

		key, value, isEnd := scanner.ParseLine("=")
		if isEnd {
			break
		}

		if len(key) > prefixLen && key[:prefixLen] == prefix {
			key = key[prefixLen:]
		}

		setLLDPField(&p.LLDP, strings.TrimSpace(key), strings.TrimSpace(value))

	}

	return scanner.Err()
}

// setLLDPField maps a stripped LLDP key to the appropriate LLDP struct field.
func setLLDPField(l *LLDP, key, value string) {
	switch key {
	case fieldChassisMac:
		l.ToRAddress = value
	case fieldChassisName:
		l.ToRName = value
	case fieldChassisMgmtIP:
		l.ManagementIP = value
	case fieldPortIfname:
		l.Interface = value
	case fieldPortAggregation:
		l.PortAggregation = value
	case fieldVlanID:
		l.VLAN = value
	case fieldVlanPvid:
		l.PPVID = value
	case fieldPpvidSupport:
		l.PPVIDSupport = value
	case fieldPpvidEnabled:
		l.PPVIDEnabled = value
	}
}

// parseSection identifies which section of a two-section ethtool output
// block (Pre-set / Current) is being parsed.
type parseSection int

const (
	sectionPreset  parseSection = iota // "Pre-set maximums" section
	sectionCurrent                     // "Current hardware settings" section
)

// sectionFieldSetter maps an ethtool field key to the destination pointers
// for its pre-set maximum and current value respectively.
type sectionFieldSetter struct {
	key           string
	maxSetter     *string
	currentSetter *string
}

// applySectionFields parses a two-section ethtool output block (such as ring
// buffer or channel settings) and dispatches each value to the appropriate
// destination pointer based on the active section.
func applySectionFields(data []byte, source []sectionFieldSetter) error {
	scanner := bufio.NewScanner(bytes.NewReader(data))
	section := sectionPreset

	for scanner.Scan() {
		line := scanner.Text()

		if len(line) != 0 {
			switch line[:2] {
			case "Pr":
				section = sectionPreset
				continue
			case "Cu":
				section = sectionCurrent
				continue
			}

			key, value, ok := strings.Cut(line, ":")
			if !ok {
				continue
			}

			key = strings.TrimSpace(key)
			value = strings.TrimSpace(value)

			for _, field := range source {
				if field.key == key {
					switch section {
					case sectionPreset:
						if field.maxSetter != nil {
							*field.maxSetter = value
						}
					case sectionCurrent:
						if field.currentSetter != nil {
							*field.currentSetter = value
						}
					}
				}
			}
		}
	}

	return scanner.Err()
}

// collectEthtoolRingBuffer runs `ethtool -g <iface>` and populates the
// NIC ring buffer (RX/TX) pre-set maximums and current settings.
func (p *PhyInterface) collectEthtoolRingBuffer() error {
	output, err := shell.Run("ethtool", "-g", p.DeviceName)
	if err != nil {
		return nil
	}

	return applySectionFields(output, []sectionFieldSetter{
		{key: "RX", maxSetter: &p.RingBuffer.MaxRX, currentSetter: &p.RingBuffer.CurrentRX},
		{key: "TX", maxSetter: &p.RingBuffer.MaxTX, currentSetter: &p.RingBuffer.CurrentTX},
	})
}

// collectEthtoolChannel runs `ethtool -l <iface>` and populates the NIC
// channel (queue) pre-set maximums and current configuration.
func (p *PhyInterface) collectEthtoolChannel() error {
	output, err := shell.Run("ethtool", "-l", p.DeviceName)
	if err != nil {
		return nil
	}

	return applySectionFields(
		output,
		[]sectionFieldSetter{
			{key: "Rx", maxSetter: &p.Channel.MaxRX, currentSetter: &p.Channel.CurrentRX},
			{key: "Tx", maxSetter: &p.Channel.MaxTX, currentSetter: &p.Channel.CurrentTX},
			{key: "Combined", maxSetter: &p.Channel.MaxCombined, currentSetter: &p.Channel.CurrentCombined},
		},
	)
}
