package network

import (
	"bufio"
	"bytes"
	"strings"

	"github.com/zx-cc/baize/internal/collector/pci"
	"github.com/zx-cc/baize/pkg/shell"
	"github.com/zx-cc/baize/pkg/utils"
)

func (p *PhyInterface) collectPhyFromPCI(bus string) error {
	pciInfo, err := pci.GetByBus(bus)
	if err != nil {
		return err
	}
	p.PCI = *pciInfo

}

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

func (p *PhyInterface) collectLLDPNeighbors(nic string) error {
	data, err := shell.Run("lldpctl", p.DeviceName, "-f", "keyvalue")
	if err != nil {
		return err
	}

	var prefixBuilder strings.Builder
	prefixBuilder.Grow(7 + len(nic))
	prefixBuilder.WriteString("lldp.")
	prefixBuilder.WriteString(nic)
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

		setLLDPField(&res, strings.TrimSpace(key), strings.TrimSpace(value))

	}

	if err := scanner.Err(); err != nil {
		return res, err
	}

	return res, nil
}

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

type parseSection int

const (
	sectionPreset parseSection = iota
	sectionCurrent
)

type sectionFieldSetter struct {
	key           string
	maxSetter     *string
	currentSetter *string
}

func applySectionFields(data []byte, source []sectionFieldSetter) {
	scanner := bufio.NewScanner(bytes.NewReader(data))
	section := sectionPreset

	for scanner.Scan() {
		line := scanner.Text()

		if len(line) != 0 {
			switch line[0] {
			case 'P':
				section = sectionPreset
				continue
			case 'C':
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
}

func collectEthtoolRingBuffer(nic string) RingBuffer {
	output := execute.Command(ethtool, "-g", nic)
	if output.AsError() != nil {
		return RingBuffer{}
	}

	var res RingBuffer

	applySectionFields(output.Stdout, []sectionFieldSetter{
		{key: "RX", maxSetter: &res.MaxRX, currentSetter: &res.CurrentRX},
		{key: "TX", maxSetter: &res.MaxTX, currentSetter: &res.CurrentTX},
	})

	return res
}

func collectEthtoolChannel(nic string) Channel {
	output := execute.Command(ethtool, "-l", nic)
	if output.AsError() != nil {
		return Channel{}
	}

	var res Channel
	applySectionFields(
		output.Stdout,
		[]sectionFieldSetter{
			{key: "Rx", maxSetter: &res.MaxRX, currentSetter: &res.CurrentRX},
			{key: "Tx", maxSetter: &res.MaxTX, currentSetter: &res.CurrentTX},
			{key: "Combined", maxSetter: &res.MaxCombined, currentSetter: &res.CurrentCombined},
		},
	)

	return res
}
