package network

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/zx-cc/baize/pkg/paths"
	"github.com/zx-cc/baize/pkg/utils"
)

func New() *Network {
	return &Network{
		Net:  make([]*NetInterface, 0, 16),
		Phy:  make([]*PhyInterface, 0, 8),
		Bond: make([]*BondInterface, 0, 2),
	}
}

func (n *Network) Collect() error {
	return nil
}

var skipTarget = []string{"lo", "loop", "bonding_master"}

func (n *Network) collectNetFromSysfs() error {
	netDirs, err := os.ReadDir(paths.SysClassNet)
	if err != nil {
		return err
	}

	if len(netDirs) == 0 {
		return errors.New("no net found")
	}

	names := make([]string, 0, len(netDirs))
	for _, net := range netDirs {
		name := net.Name()
		if slices.Contains(skipTarget, name) {
			continue
		}

		names = append(names, name)
	}

	if len(names) == 0 {
		return errors.New("available net not found")
	}

	errs := make([]error, 0, len(names))

	for _, name := range names {
		net, err := collectNetInf(name)
		if err != nil {
			errs = append(errs, err)
		}
		n.Net = append(n.Net, net)
	}

	return errors.Join(errs...)
}

var nic []string

func (n *Network) collectNIC() error {
	if len(nic) == 0 {
		return errors.New("nic not found")
	}

	errs := make([]error, 0, len(nic))

	for _, eth := range nic {
		phy := &PhyInterface{
			DeviceName: eth,
		}
		devicePath := filepath.Join(paths.SysClassNet, eth, "device")
		bus, err := utils.ReadLinkBase(devicePath)
		if err != nil {
			errs = append(errs, err)
			n.Phy = append(n.Phy, phy)
			continue
		}

		if err := phy.collectPhyFromPCI(bus); err != nil {
			errs = append(errs, err)
		}
	}

	return errors.Join(errs...)
}

func (n *Network) collectBondFromProc() error {
	bonds, err := os.ReadDir(paths.ProcNetBonding)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read directory %s : %w", paths.ProcNetBonding, err)
	}

	errs := make([]error, 0, len(bonds))
	for _, bond := range bonds {
		if !bond.Type().IsRegular() {
			continue
		}

		bondInterface, err := parseBondFile(bond.Name())
		if err != nil {
			errs = append(errs, err)
			continue
		}

		n.Bond = append(n.Bond, bondInterface)
	}

	return nil
}

func parseBondFile(name string) (*BondInterface, error) {
	res := &BondInterface{
		BondName:        name,
		SlaveInterfaces: make([]SlaveInterface, 0, 2),
	}

	file, err := os.Open(filepath.Join(paths.ProcNetBonding, name))
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var inSlaveSection bool
	scanner := utils.NewScanner(file)
	for {
		key, value, isEnd := scanner.ParseLine(":")
		if isEnd {
			break
		}

		switch key {
		case "Bonding Mode":
			res.BondMode = value
		case "Transmit Hash Policy":
			res.TransmitHashPolicy = value
		case "MII Polling Interval (ms)":
			res.MIIPollingInterval = value
		case "LACP Rate":
			res.LACPRate = value
		case "Aggregator ID":
			if !inSlaveSection {
				res.AggregatorID = value
			}
		case "Number of ports":
			res.NumberOfPorts = value
		case "Slave Interface":
			inSlaveSection = true
			res.SlaveInterfaces = append(res.SlaveInterfaces, parseSlaveInterface(value))
		}
	}

	return res, nil
}

var slaveFieldMap = []struct {
	name   string
	setter func(*SlaveInterface, string)
}{
	{name: "link_failure_count", setter: func(s *SlaveInterface, val string) { s.LinkFailureCount = val }},
	{name: "ad_aggregator_id", setter: func(s *SlaveInterface, val string) { s.AggregatorID = val }},
	{name: "queue_id", setter: func(s *SlaveInterface, val string) { s.QueueID = val }},
	{name: "mii_status", setter: func(s *SlaveInterface, val string) { s.MIIStatus = val }},
	{name: "state", setter: func(s *SlaveInterface, val string) { s.State = val }},
}

func parseSlaveInterface(slave string) SlaveInterface {
	res := SlaveInterface{
		SlaveName: slave,
	}

	dirPath := filepath.Join(paths.SysClassNet, slave, "bonding_slave")

	for _, field := range slaveFieldMap {
		filePath := filepath.Join(dirPath, field.name)
		content, err := os.ReadFile(filePath)
		if err != nil {
			continue
		}
		field.setter(&res, strings.TrimSpace(string(content)))
	}

	return res
}
