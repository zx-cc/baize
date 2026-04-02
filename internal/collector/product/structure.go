package product

type Product struct {
	OperatingSystem `json:"operating_system"`
	BIOS            `json:"bios"`
	System          `json:"system"`
	BaseBoard       `json:"base_board"`
	Chassis         `json:"chassis"`
}

type OperatingSystem struct {
	KernelType          string `json:"kernel_type,omitzero"`
	KernelRelease       string `json:"kernel_release,omitzero"`
	KernelVersion       string `json:"kernel_version,omitzero"`
	HostName            string `json:"host_name,omitzero"`
	Distribution        string `json:"distribution,omitzero"`
	DistributionVersion string `json:"distribution_version,omitzero"`
	IDLike              string `json:"id_like,omitzero"`
}

type BIOS struct {
	Vendor           string `json:"vendor,omitzero"`
	Version          string `json:"version,omitzero"`
	ReleaseDate      string `json:"release_date,omitzero"`
	BIOSRevision     string `json:"bios_revision,omitzero"`
	FirmwareRevision string `json:"firmware_revision,omitzero"`
}

type System struct {
	Manufacturer string `json:"manufacturer,omitzero"`
	ProductName  string `json:"product_name,omitzero"`
	SN           string `json:"serial_number,omitzero"`
	UUID         string `json:"uuid,omitzero"`
	PN           string `json:"part_number,omitzero"`
	Family       string `json:"family,omitzero"`
}

type BaseBoard struct {
	Manufacturer string `json:"manufacturer,omitzero"`
	SN           string `json:"serial_number,omitzero"`
	Type         string `json:"type,omitzero"`
}

type Chassis struct {
	Manufacturer string `json:"manufacturer,omitzero"`
	Type         string `json:"type,omitzero"`
	SN           string `json:"serial_number,omitzero"`
	AssetTag     string `json:"asset_tag,omitzero"`
	Height       string `json:"height,omitzero"`
	PN           string `json:"part_number,omitzero"`
}

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
