package smart

type SMART struct {
	ModelName   string `json:"model_name,omitempty"`
	Vendor      string `json:"vendor,omitempty"`
	PN          string `json:"part_number,omitempty"`
	Firmware    string `json:"firmware,omitempty"`
	SN          string `json:"sn,omitempty"`
	Rotation    string `json:"rotation_rate,omitempty"`
	MediaType   string `json:"media_type,omitempty"`
	FormFactor  string `json:"form_factor,omitempty"`
	Protocol    string `json:"protocol,omitempty"`
	ProtocolVer string `json:"protocol_version,omitempty"`
	Capacity    string `json:"capacity,omitempty"`
	PowerOn     string `json:"power_on_time,omitempty"`
	Temperature string `json:"temperature,omitempty"`
	WriteCache  string `json:"write_cache,omitempty"`
	ReadCache   string `json:"read_cache,omitempty"`
	SMARTStatus bool   `json:"smart_status,omitempty"`
	SMARTAttrs  any    `json:"smart_attributes,omitempty"`
}
