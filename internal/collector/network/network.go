// Package network collects network interface information from sysfs, the PCI
// subsystem, and /proc/net/bonding.
//
// Three types of interfaces are discovered:
//   - NetInterface  — all logical network interfaces found in /sys/class/net
//   - PhyInterface  — physical NICs resolved via their PCI bus address
//   - BondInterface — bonded (LAG/LACP) interfaces parsed from /proc/net/bonding
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

// New returns an initialised Network collector with pre-allocated slices.
func New() *Network {
	return &Network{
		Net:  make([]*NetInterface, 0, 16),
		Phy:  make([]*PhyInterface, 0, 8),
		Bond: make([]*BondInterface, 0, 2),
	}
}

// Collect discovers and populates all network interface types.
// Errors from individual sub-collectors are joined and returned together.
func (n *Network) Collect() error {
	errs := make([]error, 0, 4)

	if err := n.collectNetFromSysfs(); err != nil {
		errs = append(errs, err)
	}

	if err := n.collectNIC(); err != nil {
		errs = append(errs, err)
	}

	if err := n.collectBondFromProc(); err != nil {
		errs = append(errs, err)
	}

	return errors.Join(errs...)
}

// skipTarget lists sysfs net directory entries that should be ignored during
// interface enumeration (loopback, loop-back aliases, and the bonding masters
// control file).
var skipTarget = []string{"lo", "loop", "bonding_masters"}

// collectNetFromSysfs enumerates /sys/class/net and builds the logical
// NetInterface list, skipping any entries in skipTarget.
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

		// if name == "lo" || strings.HasPrefix(name, "loop") || name == "bonding_master" {
		// 	continue
		// }

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

// nic holds the names of physical NIC interfaces identified during sysfs
// enumeration. It is populated by collectNetFromSysfs before collectNIC runs.
var nic []string

// collectNIC resolves each physical NIC name to its PCI bus address and
// collects detailed hardware information from the PCI subsystem.
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

		n.Phy = append(n.Phy, phy)
	}

	return errors.Join(errs...)
}

// collectBondFromProc reads /proc/net/bonding/* to discover all configured
// bond interfaces and their slave members.
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

// parseBondFile parses a single /proc/net/bonding/<name> file and returns
// a populated BondInterface with mode, policy, and slave details.
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

// slaveFieldMap maps sysfs bonding_slave attribute file names to their
// corresponding SlaveInterface setter functions for concise field population.
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

// parseSlaveInterface reads per-slave sysfs attributes from
// /sys/class/net/<slave>/bonding_slave/ and returns a populated SlaveInterface.
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

// Name returns the module identifier used for routing by the collector manager.
func (n *Network) Name() string {
	return "NETWORK"
}

// Jprintln serialises the collected network data to JSON and writes it to stdout.
func (n *Network) Jprintln() error {
	return utils.JSONPrintln(n)
}

// Sprintln prints a brief network summary to stdout.
func (n *Network) Sprintln() {}

// Lprintln prints a detailed network report to stdout.
func (n *Network) Lprintln() {}
