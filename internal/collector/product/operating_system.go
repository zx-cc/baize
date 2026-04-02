package product

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/zx-cc/baize/pkg/utils"
)

const (
	hostNamePath      = "/proc/sys/kernel/hostname"
	ostypePath        = "/proc/sys/kernel/ostype"
	kernelReleasePath = "/proc/sys/kernel/osrelease"
	kernelVersionPath = "/proc/sys/kernel/version"
	osReleasePath     = "/etc/os-release"
	centosReleasePath = "/etc/centos-release"
	redhatReleasePath = "/etc/redhat-release"
	rockyReleasePath  = "/etc/rocky-release"
	debianVersionPath = "/etc/debian_version"
)

func collectOS() (*OperatingSystem, error) {
	res := &OperatingSystem{}

	kernelCfgs := []struct {
		path  string
		value *string
	}{
		{path: ostypePath, value: &res.KernelType},
		{path: kernelReleasePath, value: &res.KernelRelease},
		{path: kernelVersionPath, value: &res.KernelVersion},
		{path: hostNamePath, value: &res.HostName},
	}

	errs := make([]error, 0, len(kernelCfgs))

	for _, cfg := range kernelCfgs {
		content, err := os.ReadFile(cfg.path)
		if err != nil {
			errs = append(errs, fmt.Errorf("read %s: %w", cfg.path, err))
			*cfg.value = "Unknown"
			continue
		}

		*cfg.value = string(bytes.TrimSpace(content))
	}

	return res, errors.Join(errs...)
}

func collectDistribution(m *OperatingSystem) error {
	file, err := os.Open(osReleasePath)
	if err != nil {
		return fmt.Errorf("open file %s: %w", osReleasePath, err)
	}
	defer file.Close()

	scanner := utils.NewScanner(file)
	for {
		key, value, isEnd := scanner.ParseLine("=")
		if isEnd {
			break
		}
		switch key {
		case "NAME":
			m.Distribution = value
		case "ID_LIKE":
			m.IDLike = value
		}
	}

	m.DistributionVersion = getDistributionVersion(m.Distribution)

	return nil
}

var (
	regexVersion = regexp.MustCompile(`[\( ]([\d\.]+)`) // Ubuntu, RHEL 通用
	regexCentos  = regexp.MustCompile(`^CentOS(?: Linux)? release ([\d\.]+)`)
	regexRocky   = regexp.MustCompile(`^Rocky Linux release ([\d\.]+)`)
	regexDebian  = regexp.MustCompile(`^([\d\.]+)`)

	distrMatchers = []struct {
		prefix   string
		filePath string
		regex    *regexp.Regexp
		submatch int
	}{
		{prefix: "ubuntu", regex: regexVersion, submatch: 1},
		{prefix: "centos", filePath: centosReleasePath, regex: regexCentos, submatch: 1},
		{prefix: "rocky", filePath: rockyReleasePath, regex: regexRocky, submatch: 1},
		{prefix: "debian", filePath: debianVersionPath, regex: regexDebian, submatch: 1},
		{prefix: "rhel", filePath: redhatReleasePath, regex: regexVersion, submatch: 1},
		{prefix: "red hat", filePath: redhatReleasePath, regex: regexVersion, submatch: 1},
	}
)

func getDistributionVersion(ver string) string {
	ver = strings.ToLower(ver)
	if ver == "" {
		return "Unknown"
	}

	for _, matcher := range distrMatchers {
		if !strings.Contains(ver, matcher.prefix) {
			continue
		}
		var content []byte
		if matcher.filePath != "" {
			var err error
			content, err = os.ReadFile(matcher.filePath)
			if err != nil {
				continue
			}
		}

		if matches := matcher.regex.FindSubmatch(content); len(matches) > matcher.submatch {
			return string(matches[matcher.submatch])
		}
	}

	return "Unknown"
}
