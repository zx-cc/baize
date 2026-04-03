// Package memory provides data structures for memory runtime statistics,
// SMBIOS DIMM inventory, and EDAC error counters.
package memory

// Memory is the top-level container for all memory-related information
// collected from /proc/meminfo, SMBIOS type-17, and the EDAC subsystem.
type Memory struct {
	PhysicalMemorySize    string               `json:"physical_memory_size,omitempty" name:"Physical Memory" output:"both" color:"defaultGreen"`
	Maxslots              string               `json:"max_slots,omitempty" name:"Slot Max" output:"both"`
	UsedSlots             string               `json:"used_slots,omitempty" name:"Slot Used" output:"both"`
	MemTotal              string               `json:"memory_total,omitempty" name:"System Memory" output:"both"`
	MemFree               string               `json:"memory_free,omitempty" name:"Memory Free" output:"both"`
	MemAvailable          string               `json:"memory_available,omitempty" name:"Memory Available" output:"both"`
	SwapCached            string               `json:"swap_cached,omitempty"`
	SwapTotal             string               `json:"swap_total,omitempty" name:"Swap" output:"both"`
	SwapFree              string               `json:"swap_free,omitempty"`
	Buffer                string               `json:"buffer,omitempty" name:"Buffer" output:"both"`
	Cached                string               `json:"cached,omitempty" name:"Cached" output:"both"`
	Slab                  string               `json:"slab,omitempty"`
	SReclaimable          string               `json:"s_reclaimable,omitempty"`
	SUnreclaim            string               `json:"s_unreclaim,omitempty"`
	KReclaimable          string               `json:"k_reclaimable,omitempty"`
	KernelStack           string               `json:"kernel_stack,omitempty"`
	PageTables            string               `json:"page_tables,omitempty"`
	Dirty                 string               `json:"dirty,omitempty"`
	Writeback             string               `json:"writeback,omitempty"`
	HPagesTotal           string               `json:"huge_page_total,omitempty"`
	HPageSize             string               `json:"huge_page_size,omitempty"`
	HugeTlb               string               `json:"huge_tlb,omitempty"`
	Diagnose              string               `json:"diagnose,omitempty" name:"Diagnose" output:"both" color:"Diagnose"`
	DiagnoseDetail        string               `json:"diagnose_detail,omitempty" name:"Diagnose Detail" output:"both" color:"Diagnose"`
	EdacSlots             string               `json:"edac_slots,omitempty"`
	EdacMemorySize        string               `json:"edac_memory_size,omitempty"`
	PhysicalMemoryEntries []*SmbiosMemoryEntry `json:"physical_memory_entries,omitempty" output:"detail"`
	EdacMemoryEntries     []*EdacMemoryEntry   `json:"edac_memory_entries,omitempty"`
}

// SmbiosMemoryEntry holds the physical attributes of a single DIMM slot as
// reported by SMBIOS type-17 (Memory Device) tables.
type SmbiosMemoryEntry struct {
	Size              string `json:"size,omitempty" name:"Size" output:"detail"`
	DeviceType        string `json:"device_type,omitempty" name:"Device Type" output:"detail"`
	SerialNumber      string `json:"serial_number,omitempty" name:"SN" output:"detail"`
	Manufacturer      string `json:"manufacturer,omitempty" name:"Manufacturer" output:"detail"`
	TotalWidth        string `json:"total_width,omitempty" name:"Total Width" output:"detail"`
	DataWidth         string `json:"data_width,omitempty" name:"Data Width" output:"detail"`
	FormFactor        string `json:"form_factor,omitempty" name:"Form Factor" output:"detail"`
	DeviceLocator     string `json:"device_locator,omitempty" name:"Device Locator" output:"detail"`
	BankLocator       string `json:"bank_locator,omitempty" name:"Bank Locator" output:"detail"`
	Type              string `json:"type,omitempty" name:"Type" output:"detail"`
	TypeDetail        string `json:"type_detail,omitempty"`
	Speed             string `json:"speed,omitempty" name:"Speed" output:"detail"`
	PartNumber        string `json:"part_number,omitempty"`
	Rank              string `json:"rank,omitempty" name:"Rank" output:"detail"`
	ConfiguredSpeed   string `json:"configured_speed,omitempty"`
	ConfiguredVoltage string `json:"configured_voltage,omitempty"`
	Technology        string `json:"technology,omitempty"`
}

// EdacMemoryEntry holds the ECC error counters and topology identifiers for a
// single DIMM as reported by the Linux EDAC kernel subsystem.
type EdacMemoryEntry struct {
	Size                string `json:"size,omitempty"`
	DeviceType          string `json:"device_type,omitempty"`
	SerialNumber        string `json:"serial_number,omitempty"`
	Manufacturer        string `json:"manufacturer,omitempty"`
	CorrectableErrors   string `json:"correctable_errors,omitempty"`
	UncorrectableErrors string `json:"uncorrectable_errors,omitempty"`
	EdacMode            string `json:"edac_mode,omitempty"`
	MemoryLocation      string `json:"memory_location,omitempty"`
	MemoryType          string `json:"memory_type,omitempty"`
	SocketID            string `json:"socket_id,omitempty"`
	MemoryControllerID  string `json:"memory_controller_id,omitempty"`
	ChannelID           string `json:"channel_id,omitempty"`
	DIMMID              string `json:"dimm_id,omitempty"`
}
