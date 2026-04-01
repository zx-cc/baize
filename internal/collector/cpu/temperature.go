package cpu

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/zx-cc/baize/pkg/paths"
)

func findHwmonDir() ([]string, error) {
	dirs, err := os.ReadDir(paths.SysClassHwmon)
	if err != nil {
		return nil, err
	}

	var hwmonDirs []string
	for _, dir := range dirs {
		namePath := filepath.Join(paths.SysClassHwmon, dir.Name(), "name")
		name, err := os.ReadFile(namePath)
		if err != nil {
			continue
		}

		if bytes.Contains(name, []byte("coretemp")) || bytes.Contains(name, []byte("k10temp")) {
			hwmonDirs = append(hwmonDirs, filepath.Join(paths.SysClassHwmon, dir.Name()))
		}
	}

	return hwmonDirs, nil
}

func collectTempFromHwmon() (map[string]int, error) {
	hwmonDirs, err := findHwmonDir()
	if err != nil {
		return nil, err
	}

	if len(hwmonDirs) == 0 {
		return nil, fmt.Errorf("no coretemp hwmon device found under %s", paths.SysClassHwmon)
	}

	res := make(map[string]int)
	for _, hwmonDir := range hwmonDirs {
		labels, err := filepath.Glob(filepath.Join(hwmonDir, "temp*_label"))
		if err != nil {
			continue
		}
		var pid string
		type tempEntry struct {
			id    string
			value int
		}
		temp := make([]tempEntry, 0, len(labels))

		for _, label := range labels {
			content, err := os.ReadFile(label)
			if err != nil {
				continue
			}
			coreTemp := tempEntry{}
			switch {
			case bytes.HasPrefix(content, []byte("Package id")):
				pid = string(bytes.TrimSpace(content[11:]))
				coreTemp.id = pid
			case bytes.HasPrefix(content, []byte("Core")):
				coreTemp.id = string(bytes.TrimSpace(content[4:]))
			case bytes.HasPrefix(content, []byte("Tctl")):
				pid = string(bytes.TrimSpace(content[4:]))
				if pid == "" {
					pid = "0"
				}
				coreTemp.id = pid
			case bytes.HasPrefix(content, []byte("Tccd")):
				coreTemp.id = string(bytes.TrimSpace(content[4:]))
			}

			inputFile := strings.Replace(label, "_label", "_input", 1)
			inputValue, err := os.ReadFile(inputFile)
			if err != nil {
				continue
			}
			val, err := strconv.Atoi(strings.TrimSpace(string(inputValue)))
			if err != nil {
				continue
			}
			coreTemp.value = val / 1000
			temp = append(temp, coreTemp)
		}
		for _, t := range temp {
			res[fmt.Sprintf("%s-%s", pid, t.id)] = t.value
		}
	}

	return res, nil
}

func collectTempFromIpmitool() (map[string]int, error) {
	return nil, nil
}
