package pci

type PCI struct {
	ID          string `json:"pci_id,omitzero"`
	Bus         string `json:"pci_bus,omitzero"`
	VendorID    string `json:"vendor_id,omitzero"`
	DeviceID    string `json:"device_id,omitzero"`
	SubVendorID string `json:"sub_vendor_id,omitzero"`
	SubDeviceID string `json:"sub_device_id,omitzero"`
	ClassID     string `json:"class_id,omitzero"`
	SubClassID  string `json:"sub_class_id,omitzero"`

	Vendor    string  `json:"vendor,omitzero"`
	Device    string  `json:"device,omitzero"`
	SubVendor string  `json:"sub_vendor,omitzero"`
	SubDevice string  `json:"sub_device,omitzero"`
	Class     string  `json:"class,omitzero"`
	SubClass  string  `json:"sub_class,omitzero"`
	ProgIfID  string  `json:"prog_interface_id,omitzero"`
	Numa      string  `json:"numa,omitzero"`
	Revision  string  `json:"revision,omitzero"`
	Driver    *Driver `json:"driver,omitzero"`
	Link      *Link   `json:"link,omitzero"`
}

// Driver 表示PCI设备的驱动信息
type Driver struct {
	DriverName string `json:"driver_name,omitzero"`    // 驱动名称
	DriverVer  string `json:"driver_version,omitzero"` // 驱动版本
	SrcVer     string `json:"src_version,omitzero"`    // 源版本
	FileName   string `json:"file_name,omitzero"`      // 文件名
}

// Link 表示PCI设备的链接信息
type Link struct {
	MaxSpeed  string `json:"max_link_speed,omitzero"`     // 最大链接速度
	MaxWidth  string `json:"max_link_width,omitzero"`     // 最大链接宽度
	CurrSpeed string `json:"current_link_speed,omitzero"` // 当前链接速度
	CurrWidth string `json:"current_link_width,omitzero"` // 当前链接宽度
}
