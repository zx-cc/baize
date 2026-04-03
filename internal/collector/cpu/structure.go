package cpu

// CPU holds system-wide processor information aggregated from lscpu,
// SMBIOS type-4 tables, and the sysfs hwmon thermal subsystem.
type CPU struct {
	ModelName    string `json:"model_name,omitzero"`   // Processor model name string
	Vendor       string `json:"vendor,omitzero"`       // Normalised vendor name (e.g. "Intel", "AMD")
	Architecture string `json:"architecture,omitzero"` // ISA architecture (e.g. "x86_64", "aarch64")

	Family   string `json:"family,omitzero"`   // CPU family identifier
	Model    string `json:"model,omitzero"`    // CPU model identifier
	Stepping string `json:"stepping,omitzero"` // Stepping / revision level
	BogoMIPS string `json:"bogomips,omitzero"` // Synthetic BogoMIPS rating from the kernel

	Sockets        string `json:"socket(s),omitzero"`          // Number of physical processor sockets
	CoresPerSocket string `json:"cores_per_socket,omitzero"`   // Physical cores per socket
	ThreadsPerCore string `json:"threads_per_socket,omitzero"` // Hardware threads per core (SMT)
	CPUs           string `json:"cpu(s),omitzero"`             // Total logical CPU count (online + offline)
	OnlineCPUs     string `json:"online_cpu(s),omitzero"`      // Currently online logical CPU list

	OpMode         string `json:"op_mode,omitzero"`        // Supported operating modes (e.g. "32-bit, 64-bit")
	AddressSize    string `json:"address_size,omitzero"`   // Physical / virtual address bus widths
	ByteOrder      string `json:"byte_order,omitzero"`     // Endianness ("Little Endian" / "Big Endian")
	Virtualization string `json:"virtualization,omitzero"` // Hardware virtualisation extension (VT-x / AMD-V)
	L1dCache       string `json:"l1d_cache,omitzero"`      // L1 data cache size
	L1iCache       string `json:"l1i_cache,omitzero"`      // L1 instruction cache size
	L2Cache        string `json:"l2_cache,omitzero"`       // L2 unified cache size
	L3Cache        string `json:"l3_cache,omitzero"`       // L3 shared cache size

	HT         string `json:"Hyper_threading,omitzero"` // Hyper-Threading / SMT enabled status
	PowerState string `json:"power_state,omitzero"`     // Current CPU power governor state
	BaseFreq   string `json:"base_frequency,omitzero"`  // Base (rated) clock frequency
	MinFreq    string `json:"min_frequency,omitzero"`   // Minimum frequency under power scaling
	MaxFreq    string `json:"max_frequency,omitzero"`   // Maximum turbo frequency
	Temp       string `json:"temperature,omitzero"`     // Package-level temperature
	Watt       string `json:"watt,omitzero"`            // Package TDP / power draw

	Diagnose       string `json:"diagnose,omitzero"`        // Health summary (e.g. "Healthy")
	DiagnoseDetail string `json:"diagnose_detail,omitzero"` // Human-readable anomaly description

	Flags []string `json:"flags,omitzero"` // CPU feature flags (from lscpu / /proc/cpuinfo)

	PhysicalCPUs []*PhysicalCPU `json:"physical_cpu,omitzero"` // Per-socket physical CPU descriptors
	threads      []*Thread      // Internal per-logical-thread topology (not exported)
}

// PhysicalCPU describes a single physical processor socket as reported by
// the SMBIOS type-4 table, plus per-thread thermal and topology data.
type PhysicalCPU struct {
	Designation     string    `json:"designation,omitzero"`     // Socket label (e.g. "CPU1", "P0")
	Type            string    `json:"type,omitzero"`            // Processor type (e.g. "Central Processor")
	Family          string    `json:"family,omitzero"`          // Processor family string
	Vendor          string    `json:"vendor,omitzero"`          // Manufacturer name from SMBIOS
	Version         string    `json:"version,omitzero"`         // Firmware version / model string
	Voltage         string    `json:"voltage,omitzero"`         // Operating voltage
	ExternalClock   string    `json:"external_clock,omitzero"`  // External (front-side bus) clock
	Status          string    `json:"status,omitzero"`          // Socket population / CPU status
	Upgrade         string    `json:"upgrade,omitzero"`         // Upgrade type (e.g. "LGA1151")
	CoreCount       string    `json:"core_count,omitzero"`      // Physical core count
	CoreEnabled     string    `json:"core_enabled,omitzero"`    // Number of enabled cores
	ThreadCount     string    `json:"thread_count,omitzero"`    // Total hardware thread count
	Characteristics []string  `json:"characteristics,omitzero"` // Capability flags from SMBIOS
	Threads         []*Thread `json:"thread_entry,omitzero"`    // Per-thread topology and temperature
}

// Thread represents a single logical CPU thread with its topology IDs and
// real-time frequency and temperature readings.
type Thread struct {
	PID  string `json:"physical_id,omitzero"` // Physical socket ID
	DID  string `json:"die_id,omitzero"`      // Die ID within the socket (AMD)
	CID  string `json:"core_id,omitzero"`     // Physical core ID within the die
	TID  string `json:"thread_id,omitzero"`   // Logical thread index within the core
	Freq string `json:"frequency,omitzero"`   // Current operating frequency
	Temp string `json:"temperature,omitzero"` // Per-core temperature from hwmon
}
