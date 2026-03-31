package pci

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/zx-cc/baize/pkg/paths"
)

var pciIDSPaths = []string{
	"/usr/share/hwdata/pci.ids",
	"/usr/share/misc/pci.ids",
	"/usr/share/pci.ids",
	"/var/lib/pciutils/pci.ids",
	"/usr/local/share/pci.ids",
}

type vendorInfo struct {
	ID      string
	Name    string
	Devices map[string]*deviceInfo
}

type deviceInfo struct {
	ID         string
	Name       string
	Subsystems map[string]*subsystemInfo
}

type subsystemInfo struct {
	SubvendorID string
	SubdeviceID string
	Name        string
}

type classInfo struct {
	ID         string
	Name       string
	Subclasses map[string]*subclassInfo
}

type subclassInfo struct {
	ID         string
	Name       string
	ProgIfaces map[string]string
}

type pciDatabase struct {
	vendors map[string]*vendorInfo // [vendor_id] -> vendorInfo
	classes map[string]*classInfo  // [class_id] -> classInfo
	loaded  bool
	mu      sync.RWMutex
	path    string // pci.ids path
}

var (
	defaultDB     *pciDatabase
	defaultDBOnce sync.Once
)

func GetPCIDatabase() *pciDatabase {
	defaultDBOnce.Do(func() {
		defaultDB = NewPCIDatabase()
		defaultDB.Load()
	})

	return defaultDB
}

func NewPCIDatabase() *pciDatabase {
	return &pciDatabase{
		vendors: make(map[string]*vendorInfo),
		classes: make(map[string]*classInfo),
	}
}

func (db *pciDatabase) Load() error {
	db.mu.Lock()
	defer db.mu.Unlock()

	if db.loaded {
		return nil
	}

	var path string
	for _, p := range pciIDSPaths {
		if paths.Exists(p) {
			path = p
			break
		}
	}

	if path == "" {
		return errors.New("pci.ids not found")
	}

	return db.loadFromPath(path)
}

func (db *pciDatabase) LoadFromPath(path string) error {
	db.mu.Lock()
	defer db.mu.Unlock()
	return db.loadFromPath(path)
}

func (db *pciDatabase) loadFromPath(path string) error {
	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("failed to open pci.ids: %w", err)
	}
	defer file.Close()

	db.path = path
	db.vendors = make(map[string]*vendorInfo)
	db.classes = make(map[string]*classInfo)

	scanner := bufio.NewScanner(file)
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024)

	var currentVendor *vendorInfo
	var currentDevice *deviceInfo
	var currentClass *classInfo
	var currentSubclass *subclassInfo
	inClassSection := false

	for scanner.Scan() {
		line := scanner.Text()

		if len(line) == 0 || line[0] == '#' {
			continue
		}

		if line == "# List of known device classes, subclasses and interfaces" ||
			strings.HasPrefix(line, "C ") {
			inClassSection = true
		}

		if inClassSection {
			db.parseClassLine(line, &currentClass, &currentSubclass)
		} else {
			db.parseVendorLine(line, &currentVendor, &currentDevice)
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("scanning pci.ids: %w", err)
	}

	db.loaded = true
	return nil
}

func (db *pciDatabase) parseVendorLine(line string, currentVendor **vendorInfo, currentDevice **deviceInfo) {
	if len(line) < 4 {
		return
	}

	switch {
	case line[0] != '\t':
		if len(line) < 6 {
			return
		}
		vendorID := strings.ToLower(line[:4])
		name := strings.TrimSpace(line[4:])

		vendor := &vendorInfo{
			ID:      vendorID,
			Name:    name,
			Devices: make(map[string]*deviceInfo),
		}
		db.vendors[vendorID] = vendor
		*currentVendor = vendor
		*currentDevice = nil
	case line[0] == '\t' && len(line) > 1 && line[1] != '\t':
		if *currentVendor == nil || len(line) < 7 {
			return
		}
		trimmed := line[1:]
		if len(trimmed) < 6 {
			return
		}
		deviceID := strings.ToLower(trimmed[:4])
		name := strings.TrimSpace(trimmed[4:])

		device := &deviceInfo{
			ID:         deviceID,
			Name:       name,
			Subsystems: make(map[string]*subsystemInfo),
		}
		(*currentVendor).Devices[deviceID] = device
		*currentDevice = device
	case len(line) > 2 && line[0] == '\t' && line[1] == '\t':
		if *currentDevice == nil {
			return
		}

		trimmed := line[2:]
		if len(trimmed) < 10 {
			return
		}
		parts := strings.Fields(trimmed)
		if len(parts) < 2 {
			return
		}
		subvendorID := strings.ToLower(parts[0])
		subdeviceID := strings.ToLower(parts[1])
		name := ""
		if len(trimmed) > 10 {
			name = strings.TrimSpace(trimmed[10:])
		}

		key := subvendorID + ":" + subdeviceID
		(*currentDevice).Subsystems[key] = &subsystemInfo{
			SubvendorID: subvendorID,
			SubdeviceID: subdeviceID,
			Name:        name,
		}
	}
}

func (db *pciDatabase) parseClassLine(line string, currentClass **classInfo, currentSubclass **subclassInfo) {
	if len(line) < 3 {
		return
	}

	switch {
	case strings.HasPrefix(line, "C "):
		if len(line) < 6 {
			return
		}
		classID := strings.ToLower(line[2:4])
		name := strings.TrimSpace(line[4:])
		class := &classInfo{
			ID:   classID,
			Name: name,
		}
		db.classes[classID] = class
		*currentClass = class
		*currentSubclass = nil
	case line[0] == '\t' && len(line) > 1 && line[1] != '\t':
		if *currentClass == nil {
			return
		}
		trimmed := line[1:]
		if len(trimmed) < 4 {
			return
		}
		subclassID := strings.ToLower(trimmed[:2])
		name := strings.TrimSpace(trimmed[2:])
		subclass := &subclassInfo{
			ID:         subclassID,
			Name:       name,
			ProgIfaces: make(map[string]string),
		}
		(*currentClass).Subclasses[subclassID] = subclass
		*currentSubclass = subclass
	case len(line) > 2 && line[0] == '\t' && line[1] == '\t':
		if *currentSubclass == nil {
			return
		}
		trimmed := line[2:]
		if len(trimmed) < 4 {
			return
		}
		preIfID := strings.ToLower(trimmed[:2])
		name := strings.TrimSpace(trimmed[2:])
		(*currentSubclass).ProgIfaces[preIfID] = name
	}
}

func (db *pciDatabase) LookupVendor(vendorID string) *vendorInfo {
	db.mu.RLock()
	defer db.mu.RUnlock()

	vendorID = strings.ToLower(strings.TrimPrefix(vendorID, "0x"))
	return db.vendors[vendorID]
}

func (db *pciDatabase) LookupDevice(vendorID, deviceID string) *deviceInfo {
	vendor := db.LookupVendor(vendorID)
	if vendor == nil {
		return nil
	}

	db.mu.RLock()
	defer db.mu.RUnlock()

	deviceID = strings.ToLower(strings.TrimPrefix(deviceID, "0x"))
	return vendor.Devices[deviceID]
}

func (db *pciDatabase) LookupSubsystem(vendorID, deviceID, subvendorID, subdeviceID string) *subsystemInfo {
	device := db.LookupDevice(vendorID, deviceID)
	if device == nil {
		return nil
	}

	db.mu.RLock()
	defer db.mu.RUnlock()

	subvendorID = strings.ToLower(strings.TrimPrefix(subvendorID, "0x"))
	subdeviceID = strings.ToLower(strings.TrimPrefix(subdeviceID, "0x"))
	key := subvendorID + ":" + subdeviceID
	return device.Subsystems[key]
}

func (db *pciDatabase) LookupClass(classID string) *classInfo {
	db.mu.RLock()
	defer db.mu.RUnlock()

	classID = strings.ToLower(strings.TrimPrefix(classID, "0x"))
	if len(classID) >= 2 {
		classID = classID[:2]
	}
	return db.classes[classID]
}

func (db *pciDatabase) LookupSubclass(classID, subclassID string) *subclassInfo {
	class := db.LookupClass(classID)
	if class == nil {
		return nil
	}

	db.mu.RLock()
	defer db.mu.RUnlock()

	subclassID = strings.ToLower(strings.TrimPrefix(subclassID, "0x"))
	if len(subclassID) >= 2 {
		subclassID = subclassID[:2]
	}

	return class.Subclasses[subclassID]
}

func (db *pciDatabase) GetVendorName(vendorID string) string {
	if vendor := db.LookupVendor(vendorID); vendor != nil {
		return vendor.Name
	}

	return ""
}

func (db *pciDatabase) GetDeviceName(vendorID, deviceID string) string {
	if device := db.LookupDevice(vendorID, deviceID); device != nil {
		return device.Name
	}

	return ""
}

func (db *pciDatabase) GetClassName(classID string) string {
	if class := db.LookupClass(classID); class != nil {
		return class.Name
	}

	return ""
}

func (db *pciDatabase) GetSubclassName(classID, subclassID string) string {
	if subclass := db.LookupSubclass(classID, subclassID); subclass != nil {
		return subclass.Name
	}

	return ""
}

func (db *pciDatabase) GetSubsystemName(vendorID, deviceID, subvendorID, subdeviceID string) string {
	if subsystem := db.LookupSubsystem(vendorID, deviceID, subvendorID, subdeviceID); subsystem != nil {
		return subsystem.Name
	}

	return ""
}

func (db *pciDatabase) IsLoaded() bool {
	db.mu.RLock()
	defer db.mu.RUnlock()
	return db.loaded
}

func (db *pciDatabase) Path() string {
	db.mu.RLock()
	defer db.mu.RUnlock()

	return db.path
}

func (db *pciDatabase) Stats() (vendors, devices, classes int) {
	db.mu.RLock()
	defer db.mu.RUnlock()

	vendors = len(db.vendors)
	classes = len(db.classes)

	for _, v := range db.vendors {
		devices += len(v.Devices)
	}

	return
}
