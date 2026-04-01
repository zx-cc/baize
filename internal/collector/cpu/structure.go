package cpu

type CPU struct {
	ModelName    string `json:"model_name,omitzero"`
	Vendor       string `json:"vendor,omitzero"`
	Architecture string `json:"architecture,omitzero"`

	Family   string `json:"family,omitzero"`
	Model    string `json:"model,omitzero"`
	Stepping string `json:"stepping,omitzero"`
	BogoMIPS string `json:"bogomips,omitzero"`

	Sockets        string `json:"socket(s),omitzero"`
	CoresPerSocket string `json:"cores_per_socket,omitzero"`
	ThreadsPerCore string `json:"threads_per_socket,omitzero"`
	CPUs           string `json:"cpu(s),omitzero"`
	OnlineCPUs     string `json:"online_cpu(s),omitzero"`

	OpMode         string `json:"op_mode,omitzero"`
	AddressSize    string `json:"address_size,omitzero"`
	ByteOrder      string `json:"byte_order,omitzero"`
	Virtualization string `json:"virtualization,omitzero"`
	L1dCache       string `json:"l1d_cache,omitzero"`
	L1iCache       string `json:"l1i_cache,omitzero"`
	L2Cache        string `json:"l2_cache,omitzero"`
	L3Cache        string `json:"l3_cache,omitzero"`

	HT         string `json:"Hyper_threading,omitzero"`
	PowerState string `json:"power_state,omitzero"`
	BaseFreq   string `json:"base_frequency,omitzero"`
	MinFreq    string `json:"min_frequency,omitzero"`
	MaxFreq    string `json:"max_frequency,omitzero"`
	Temp       string `json:"temperature,omitzero"`
	Watt       string `json:"watt,omitzero"`

	Diagnose       string `json:"diagnose,omitzero"`
	DiagnoseDetail string `json:"diagnose_detail,omitzero"`

	Flags []string `json:"flags,omitzero"`

	PhysicalCPUs []*PhysicalCPU `json:"physical_cpu,omitzero"`
	threads      []*Thread
}

type PhysicalCPU struct {
	Designation     string    `json:"designation,omitzero"`
	Type            string    `json:"type,omitzero"`
	Family          string    `json:"family,omitzero"`
	Vendor          string    `json:"vendor,omitzero"`
	Version         string    `json:"version,omitzero"`
	Voltage         string    `json:"voltage,omitzero"`
	ExternalClock   string    `json:"external_clock,omitzero"`
	Status          string    `json:"status,omitzero"`
	Upgrade         string    `json:"upgrade,omitzero"`
	CoreCount       string    `json:"core_count,omitzero"`
	CoreEnabled     string    `json:"core_enabled,omitzero"`
	ThreadCount     string    `json:"thread_count,omitzero"`
	Characteristics []string  `json:"characteristics,omitzero"`
	Threads         []*Thread `json:"thread_entry,omitzero"`
}

type Thread struct {
	PID  string `json:"physical_id,omitzero"`
	DID  string `json:"die_id,omitzero"`
	CID  string `json:"core_id,omitzero"`
	TID  string `json:"thread_id,omitzero"`
	Freq string `json:"frequency,omitzero"`
	Temp string `json:"temperature,omitzero"`
}
