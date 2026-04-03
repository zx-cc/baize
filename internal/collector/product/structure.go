// Package product provides data structures for server platform identity
// information collected from SMBIOS firmware tables and the OS.
package product

// Product is the top-level container holding all server platform identity data.
type Product struct {
	OS        *OperatingSystem `json:"operating_system"`
	BIOS      *BIOS            `json:"bios"`
	System    *System          `json:"system"`
	BaseBoard *BaseBoard       `json:"base_board"`
	Chassis   *Chassis         `json:"chassis"`
}

// OperatingSystem holds runtime OS identification fields read from
// /proc/sys/kernel/* and /etc/os-release.
type OperatingSystem struct {
	KernelType          string `json:"kernel_type,omitzero"`
	KernelRelease       string `json:"kernel_release,omitzero"`
	KernelVersion       string `json:"kernel_version,omitzero"`
	HostName            string `json:"host_name,omitzero"`
	Distribution        string `json:"distribution,omitzero"`
	DistributionVersion string `json:"distribution_version,omitzero"`
	IDLike              string `json:"id_like,omitzero"`
}

// BIOS holds firmware identification fields from SMBIOS type-0.
type BIOS struct {
	Vendor           string `json:"vendor,omitzero"`
	Version          string `json:"version,omitzero"`
	ReleaseDate      string `json:"release_date,omitzero"`
	BIOSRevision     string `json:"bios_revision,omitzero"`
	FirmwareRevision string `json:"firmware_revision,omitzero"`
}

// System holds system-level identification fields from SMBIOS type-1.
type System struct {
	Manufacturer string `json:"manufacturer,omitzero"`
	ProductName  string `json:"product_name,omitzero"`
	SN           string `json:"serial_number,omitzero"`
	UUID         string `json:"uuid,omitzero"`
	PN           string `json:"part_number,omitzero"`
	Family       string `json:"family,omitzero"`
}

// BaseBoard holds motherboard/baseboard fields from SMBIOS type-2.
type BaseBoard struct {
	Manufacturer string `json:"manufacturer,omitzero"`
	SN           string `json:"serial_number,omitzero"`
	Type         string `json:"type,omitzero"`
}

// Chassis holds chassis/enclosure fields from SMBIOS type-3.
type Chassis struct {
	Manufacturer string `json:"manufacturer,omitzero"`
	Type         string `json:"type,omitzero"`
	SN           string `json:"serial_number,omitzero"`
	AssetTag     string `json:"asset_tag,omitzero"`
	Height       string `json:"height,omitzero"`
	PN           string `json:"part_number,omitzero"`
}

// ProductBrief is a flattened, display-oriented view of the most important
// product identity fields used for brief terminal output.
type ProductBrief struct {
	ProductName         string `json:"-" name:"Product Name" output:"both" color:"DefaultGreen"`
	Manufacturer        string `json:"-" name:"Manufacturer" output:"both" color:"DefaultGreen"`
	SerialNumber        string `json:"-" name:"Serial Number" output:"both" color:"DefaultGreen"`
	UUID                string `json:"-" name:"UUID" output:"both"`
	AssetTag            string `json:"-" name:"Asset Tag" output:"both" color:"DefaultGreen"`
	ChassisType         string `json:"-" name:"Chassis Type" output:"both"`
	OSType              string `json:"-" name:"OS Type" output:"both" color:"DefaultGreen"`
	KernelRelease       string `json:"-" name:"Kernel Release" output:"both"`
	Distribution        string `json:"-" name:"Distribution" output:"both"`
	DistributionVersion string `json:"-" name:"Distribution Version" output:"both"`
	HostName            string `json:"-" name:"Hostname" output:"both" color:"DefaultGreen"`
}
