package cpu

import (
	"bytes"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/zx-cc/baize/pkg/paths"
	"github.com/zx-cc/baize/pkg/shell"
)

func (c *CPU) collectThreads() error {
	cpuDirs, err := filepath.Glob(filepath.Join(paths.SysDevicesSystemCPU, "cpu[0-9]*"))
	if err != nil {
		return err
	}

	for _, cpuDir := range cpuDirs {
		name := filepath.Base(cpuDir)
		thread := &Thread{
			TID: strings.TrimPrefix(name, "cpu"),
		}

		topologyDir := filepath.Join(cpuDir, "topology")
		thread.collectID(topologyDir)

		c.threads = append(c.threads, thread)
	}

	if strings.HasPrefix(c.Architecture, "aarch") {
		c.collectFreqFromScalling()
	} else {
		c.collectFreqFromTurbostat()
	}

	return nil
}

func (t *Thread) collectID(path string) {
	ids := []struct {
		file string
		val  *string
	}{
		{"physical_package_id", &t.PID},
		{"core_id", &t.CID},
		{"die_id", &t.DID},
	}

	for _, id := range ids {
		file := filepath.Join(path, id.file)
		val, err := os.ReadFile(file)
		if err != nil {
			continue
		}
		*id.val = strings.TrimSpace(string(val))
	}
}

func (c *CPU) collectFreqFromTurbostat() {
	data, err := shell.Run("turbostat", "-q", "sleep", "1")
	if err != nil {
		return
	}

	lines := bytes.Split(data, []byte("\n"))
	if len(lines) < 3 {
		return
	}

	// Line 1 (index 1) is the column header; line 2 (index 2) is the system summary.
	headers := strings.Fields(string(lines[1]))
	if len(headers) == 0 {
		return
	}

	headerIndex := make(map[string]int, len(headers))
	for i, header := range headers {
		headerIndex[header] = i
	}

	summaryLine := strings.Fields(string(lines[2]))
	if len(summaryLine) != len(headers) {
		return
	}

	baseFreq := getIntValue("TSC_MHz", summaryLine, headerIndex)
	minFreq := getIntValue("Bzy_MHz", summaryLine, headerIndex)
	maxFreq := minFreq

	// Populate package-level temperature and power from the summary line.
	if idx, ok := headerIndex["CoreTmp"]; ok && idx < len(summaryLine) {
		c.Temp = summaryLine[idx] + " °C"
	}
	if idx, ok := headerIndex["PkgWatt"]; ok && idx < len(summaryLine) {
		c.Watt = summaryLine[idx] + " W"
	}

	// Cache header indices used in the inner loop to avoid repeated map lookups.
	pkgIdx, hasPkg := headerIndex["Package"]
	coreIdx, hasCore := headerIndex["Core"]
	cpuIdx, hasCPU := headerIndex["CPU"]
	bzyIdx, hasBzy := headerIndex["Bzy_MHz"]

	freqMap := make(map[string]string)

	for _, line := range lines[3:] {
		parts := strings.Fields(string(line))
		if len(parts) == 0 {
			continue
		}

		var pkgVal, coreVal, threadVal string
		if !hasPkg {
			pkgVal = "0"
		} else {
			pkgVal = parts[pkgIdx]
		}

		if hasCore && coreIdx < len(parts) {
			coreVal = parts[coreIdx]
		}
		if hasCPU && cpuIdx < len(parts) {
			threadVal = parts[cpuIdx]
		}

		var coreFreq int
		if hasBzy && bzyIdx < len(parts) {
			coreFreq, _ = strconv.Atoi(parts[bzyIdx])
		}

		if coreFreq > maxFreq {
			maxFreq = coreFreq
		}
		if coreFreq > 0 && coreFreq < minFreq {
			minFreq = coreFreq
		}

		key := pkgVal + "-" + coreVal + "-" + threadVal
		freqMap[key] = formatMHz(coreFreq)
	}

	// If the minimum busy frequency is notably above the base (TSC) frequency,
	// the CPU is running in performance governor mode.
	if minFreq-50 > baseFreq {
		c.PowerState = "Performance"
	}

	c.MaxFreq = formatMHz(maxFreq)
	c.MinFreq = formatMHz(minFreq)
	c.BaseFreq = formatMHz(baseFreq)

	for _, thread := range c.threads {
		key := thread.PID + "-" + thread.CID + "-" + thread.TID
		if freq, ok := freqMap[key]; ok {
			thread.Freq = freq
		}
	}
}

func (c *CPU) collectFreqFromScalling() {
	cpuDirs, err := filepath.Glob(filepath.Join(paths.SysDevicesSystemCPU, "cpu[0-9]+"))
	if err != nil {
		return
	}

	freqMap := make(map[string]string)

	for _, cpuDir := range cpuDirs {
		name := filepath.Base(cpuDir)
		tid := strings.TrimPrefix(name, "cpu")
		freqPath := filepath.Join(cpuDir, "cpufreq", "scaling_cur_freq")
		freqData, err := os.ReadFile(freqPath)
		if err != nil {
			continue
		}

		freq, err := strconv.Atoi(string(bytes.TrimSpace(freqData)))
		if err != nil {
			continue
		}

		freqMap[tid] = formatMHz(freq / 1000)
	}

	for _, thread := range c.threads {
		if freq, ok := freqMap[thread.TID]; ok {
			thread.Freq = freq
		}
	}
}

// getIntValue safely retrieves an integer value from a parsed turbostat line
// using the pre-built header index map. Returns -1 if the key is not found.
func getIntValue(key string, header []string, headerIndex map[string]int) int {
	if index, ok := headerIndex[key]; ok && index < len(header) {
		v, _ := strconv.Atoi(header[index])
		return v
	}
	return -1
}

func formatMHz(mhz int) string {
	return strconv.Itoa(mhz) + " MHz"
}
