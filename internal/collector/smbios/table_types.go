package smbios

// SMBIOS Type常量定义（基于SMBIOS 3.7规范）
const (
	TypeBIOSInfo               uint8 = 0   // BIOS信息
	TypeSystemInfo             uint8 = 1   // 系统信息
	TypeBaseboardInfo          uint8 = 2   // 主板信息
	TypeChassisInfo            uint8 = 3   // 机箱信息
	TypeProcessorInfo          uint8 = 4   // 处理器信息
	TypeMemoryController       uint8 = 5   // 内存控制器信息 (已废弃)
	TypeMemoryModule           uint8 = 6   // 内存模块信息 (已废弃)
	TypeCacheInfo              uint8 = 7   // 缓存信息
	TypePortConnector          uint8 = 8   // 端口连接器信息
	TypeSystemSlots            uint8 = 9   // 系统插槽
	TypeOnBoardDevices         uint8 = 10  // 板载设备信息 (已废弃)
	TypeOEMStrings             uint8 = 11  // OEM字符串
	TypeSystemConfig           uint8 = 12  // 系统配置选项
	TypeBIOSLanguage           uint8 = 13  // BIOS语言信息
	TypeGroupAssociations      uint8 = 14  // 组关联
	TypeSystemEventLog         uint8 = 15  // 系统事件日志
	TypePhysicalMemoryArray    uint8 = 16  // 物理内存阵列
	TypeMemoryDevice           uint8 = 17  // 内存设备
	Type32BitMemoryError       uint8 = 18  // 32位内存错误信息
	TypeMemoryArrayMappedAddr  uint8 = 19  // 内存阵列映射地址
	TypeMemoryDeviceMappedAddr uint8 = 20  // 内存设备映射地址
	TypeBuiltinPointing        uint8 = 21  // 内置指点设备
	TypePortableBattery        uint8 = 22  // 便携式电池
	TypeSystemReset            uint8 = 23  // 系统复位
	TypeHardwareSecurity       uint8 = 24  // 硬件安全
	TypeSystemPowerControls    uint8 = 25  // 系统电源控制
	TypeVoltageProbe           uint8 = 26  // 电压探测器
	TypeCoolingDevice          uint8 = 27  // 冷却设备
	TypeTemperatureProbe       uint8 = 28  // 温度探测器
	TypeElectricalCurrentProbe uint8 = 29  // 电流探测器
	TypeOutOfBandRemote        uint8 = 30  // 带外远程访问
	TypeBISEntryPoint          uint8 = 31  // 引导完整性服务入口点
	TypeSystemBootInfo         uint8 = 32  // 系统引导信息
	Type64BitMemoryError       uint8 = 33  // 64位内存错误信息
	TypeManagementDevice       uint8 = 34  // 管理设备
	TypeManagementDeviceComp   uint8 = 35  // 管理设备组件
	TypeManagementDeviceThres  uint8 = 36  // 管理设备阈值数据
	TypeMemoryChannel          uint8 = 37  // 内存通道
	TypeIPMIDeviceInfo         uint8 = 38  // IPMI设备信息
	TypeSystemPowerSupply      uint8 = 39  // 系统电源
	TypeAdditionalInfo         uint8 = 40  // 附加信息
	TypeOnboardDevicesExtended uint8 = 41  // 板载设备扩展信息
	TypeMgmtControllerHost     uint8 = 42  // 管理控制器主机接口
	TypeTPMDevice              uint8 = 43  // TPM设备
	TypeProcessorAdditional    uint8 = 44  // 处理器附加信息
	TypeFirmwareInventory      uint8 = 45  // 固件清单
	TypeStringProperty         uint8 = 46  // 字符串属性
	TypeInactive               uint8 = 126 // 非活动
	TypeEndOfTable             uint8 = 127 // 表结束
)

// TypeName 返回Type的名称
func TypeName(t uint8) string {
	names := map[uint8]string{
		TypeBIOSInfo:               "BIOS Information",
		TypeSystemInfo:             "System Information",
		TypeBaseboardInfo:          "Baseboard Information",
		TypeChassisInfo:            "Chassis Information",
		TypeProcessorInfo:          "Processor Information",
		TypeMemoryController:       "Memory Controller Information",
		TypeMemoryModule:           "Memory Module Information",
		TypeCacheInfo:              "Cache Information",
		TypePortConnector:          "Port Connector Information",
		TypeSystemSlots:            "System Slots",
		TypeOnBoardDevices:         "On Board Devices Information",
		TypeOEMStrings:             "OEM Strings",
		TypeSystemConfig:           "System Configuration Options",
		TypeBIOSLanguage:           "BIOS Language Information",
		TypeGroupAssociations:      "Group Associations",
		TypeSystemEventLog:         "System Event Log",
		TypePhysicalMemoryArray:    "Physical Memory Array",
		TypeMemoryDevice:           "Memory Device",
		Type32BitMemoryError:       "32-bit Memory Error Information",
		TypeMemoryArrayMappedAddr:  "Memory Array Mapped Address",
		TypeMemoryDeviceMappedAddr: "Memory Device Mapped Address",
		TypeBuiltinPointing:        "Built-in Pointing Device",
		TypePortableBattery:        "Portable Battery",
		TypeSystemReset:            "System Reset",
		TypeHardwareSecurity:       "Hardware Security",
		TypeSystemPowerControls:    "System Power Controls",
		TypeVoltageProbe:           "Voltage Probe",
		TypeCoolingDevice:          "Cooling Device",
		TypeTemperatureProbe:       "Temperature Probe",
		TypeElectricalCurrentProbe: "Electrical Current Probe",
		TypeOutOfBandRemote:        "Out-of-band Remote Access",
		TypeBISEntryPoint:          "Boot Integrity Services Entry Point",
		TypeSystemBootInfo:         "System Boot Information",
		Type64BitMemoryError:       "64-bit Memory Error Information",
		TypeManagementDevice:       "Management Device",
		TypeManagementDeviceComp:   "Management Device Component",
		TypeManagementDeviceThres:  "Management Device Threshold Data",
		TypeMemoryChannel:          "Memory Channel",
		TypeIPMIDeviceInfo:         "IPMI Device Information",
		TypeSystemPowerSupply:      "System Power Supply",
		TypeAdditionalInfo:         "Additional Information",
		TypeOnboardDevicesExtended: "Onboard Devices Extended Information",
		TypeMgmtControllerHost:     "Management Controller Host Interface",
		TypeTPMDevice:              "TPM Device",
		TypeProcessorAdditional:    "Processor Additional Information",
		TypeFirmwareInventory:      "Firmware Inventory",
		TypeStringProperty:         "String Property",
		TypeInactive:               "Inactive",
		TypeEndOfTable:             "End-of-Table",
	}

	if name, ok := names[t]; ok {
		return name
	}
	if t >= 128 {
		return "OEM-specific"
	}
	return "Unknown"
}
