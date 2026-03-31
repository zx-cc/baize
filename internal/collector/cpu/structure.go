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
	L1dCache       string
	L1iCache       string
	L2Cache        string
	L3Cache        string

	HT         string
	PowerState string
	BaseFreq   string
	MinFreq    string
	MaxFreq    string
	Temp       string
	Watt       string

	Diagnose       string
	DiagnoseDetail string

	Flags []string

	PhysicalCPUs []*PhysicalCPU
	threads      []*Thread
}

type PhysicalCPU struct {
	Designation     string
	Type            string
	Family          string
	Vendor          string
	Version         string
	CoreCount       string
	CoreEnabled     string
	ThreadCount     string
	Characteristics string
	Threads         []*Thread
}

type Thread struct {
	PID  string
	CID  string
	TID  string
	Freq string
	Temp string
}
