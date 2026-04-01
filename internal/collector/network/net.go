package network

import (
	"bytes"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"

	"github.com/zx-cc/baize/pkg/paths"
	"github.com/zx-cc/baize/pkg/shell"
	"github.com/zx-cc/baize/pkg/utils"
)

func collectNetInf(name string) (*NetInterface, error) {
	res := &NetInterface{
		DeviceName: name,
		IsPhy:      isPhysicalPort(name),
	}

	// Read basic interface attributes from sysfs in a single pass.
	fieldMap := map[string]*string{
		"address":   &res.MACAddress,
		"mtu":       &res.MTU,
		"duplex":    &res.Duplex,
		"speed":     &res.Speed,
		"operstate": &res.Status,
	}

	errs := make([]error, 0, len(fieldMap)+2)
	for f, ptr := range fieldMap {
		fPath := filepath.Join(paths.SysClassNet, name, f)
		data, err := os.ReadFile(fPath)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		*ptr = string(bytes.TrimSpace(data))
	}

	if err := res.collectEthtoolDriver(); err != nil {
		errs = append(errs, err)
	}

	if err := res.collectEthtoolSetting(); err != nil {
		errs = append(errs, err)
	}

	// Collect IPv4 addresses only for up interfaces that are not bond slaves.
	bondSlave := filepath.Join(paths.SysClassNet, name, "bonding_slave")
	if strings.ToLower(res.Status) == "up" && !paths.Exists(bondSlave) {
		res.IPv4, _ = getIPv4(name)
	}

	return res, errors.Join(errs...)
}

func (nf *NetInterface) collectEthtoolSetting() error {
	data, err := shell.Run("ethtool", nf.DeviceName)
	if err != nil {
		return err
	}

	scanner := utils.NewScanner(bytes.NewReader(data))
	for {
		k, v, isEnd := scanner.ParseLine(":")
		if isEnd {
			break
		}
		switch k {
		case "Speed":
			nf.Speed = v
		case "Duplex":
			nf.Duplex = v
		case "Link detected":
			nf.LinkDetected = v
		case "Port":
			nf.Port = v
		}
	}

	return scanner.Err()
}

func (nf *NetInterface) collectEthtoolDriver() error {
	data, err := shell.Run("ethtool", nf.DeviceName)
	if err != nil {
		return err
	}

	scanner := utils.NewScanner(bytes.NewReader(data))
	for {
		k, v, isEnd := scanner.ParseLine(":")
		if isEnd {
			break
		}
		switch k {
		case "driver":
			nf.Driver = v
		case "version":
			nf.DriverVersion = v
		case "firmware-version":
			nf.FirmwareVersion = v
		}
	}

	return scanner.Err()
}

func isPhysicalPort(name string) bool {
	devicePath := filepath.Join(paths.SysClassNet, name, "device")

	if !paths.Exists(devicePath) {
		return false
	}

	nic = append(nic, name)

	return true
}

// getIPv4 returns all IPv4 addresses (with netmask, prefix length, and gateway)
// assigned to the named network interface.
func getIPv4(name string) ([]IPv4Address, error) {
	nf, err := net.InterfaceByName(name)
	if err != nil {
		return nil, err
	}

	addrs, err := nf.Addrs()
	if err != nil {
		return nil, err
	}

	res := make([]IPv4Address, 0, len(addrs))
	for _, addr := range addrs {
		ipNet, ok := addr.(*net.IPNet)
		if !ok || ipNet.IP.To4() == nil {
			continue
		}
		maskSize, _ := ipNet.Mask.Size()
		res = append(res, IPv4Address{
			Address:   ipNet.IP.String(),
			Netmask:   calNetmask(ipNet.Mask),
			Gateway:   calGateway(ipNet.IP.To4(), ipNet.Mask),
			PrefixLen: fmt.Sprintf("%d", maskSize),
		})
	}

	return res, nil
}

// calNetmask converts a net.IPMask to its dotted-decimal string representation.
func calNetmask(mask net.IPMask) string {
	if len(mask) != 4 {
		return ""
	}

	return fmt.Sprintf("%d.%d.%d.%d", mask[0], mask[1], mask[2], mask[3])
}

// calGateway computes the default gateway for a given IP and subnet mask by
// taking the network address and incrementing the host portion by 1.
// Returns an empty string for invalid inputs or /32 networks.
func calGateway(ip net.IP, mask net.IPMask) string {
	if len(mask) != 4 || ip.To4() == nil {
		return ""
	}

	// For a /32 or all-ones host portion, no gateway is derivable.
	if ip[3]&mask[3] == 255 {
		return ""
	}

	gateway := net.IP{
		ip[0] & mask[0],
		ip[1] & mask[1],
		ip[2] & mask[2],
		(ip[3] & mask[3]) + 1,
	}

	return gateway.String()
}
