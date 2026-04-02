package smart

import "encoding/json"

// ==== smartctl -a -j 输出通用字段	====

type smart struct {
	Smartctl        SmartctlMeta  `json:"smartctl"`
	Device          DeviceInfo    `json:"device"`
	ModelName       string        `json:"model_name"`
	ModelFamily     string        `json:"model_family"`
	SerialNumber    string        `json:"serial_number"`
	FirmwareVersion string        `json:"firmware_version"`
	UserCapacity    *UserCapacity `json:"user_capacity,omitempty"`
	RotationRate    int           `json:"rotation_rate"`
	FormFactor      *FormFactor   `json:"form_factor,omitempty"`
	SmartStatus     *SmartStatus  `json:"smart_status,omitempty"`
	Temperature     *Temperature  `json:"temperature,omitempty"`
	PowerOnTime     *PowerOnTime  `json:"power_on_time,omitempty"`
	PowerCycleCount int           `json:"power_cycle_count,omitempty"`

	// ATA/SATA 专有
	SATAVer            *SATAVersion   `json:"sata_version,omitempty"`
	ATASmartAttributes *ATASmartAttrs `json:"ata_smart_attributes,omitempty"`

	// NVMe 专有
	NVMeTotalCap    int64        `json:"nvme_total_capacity,omitempty"`
	NVMeVer         *NVMeVersion `json:"nvme_version,omitempty"`
	NVMeSmartHealth *NVMeHealth  `json:"nvme_smart_health_information_log,omitempty"`

	// SCSI/SAS 专有
	Revision                string           `json:"revision,omitempty"`
	SCSIVer                 string           `json:"scsi_version,omitempty"`
	ScsiGrownDefectList     int              `json:"scsi_grown_defect_list,omitempty"`
	ScsiErrorCounterLog     *ScsiErrorLog    `json:"scsi_error_counter_log,omitempty"`
	ScsiStartStopCycleCount *ScsiStartStop   `json:"scsi_start_stop_cycle_counter,omitempty"`
	ScsiTemperature         *ScsiTemperature `json:"scsi_temperature,omitempty"`
}

type SmartctlMeta struct {
	Version    []int  `json:"version"`
	ExitStatus int    `json:"exit_status"`
	Messages   []SMsg `json:"messages,omitempty"`
}
type SMsg struct {
	String   string `json:"string"`
	Severity string `json:"severity"`
}
type DeviceInfo struct {
	Name     string `json:"name"`
	InfoName string `json:"info_name"`
	Type     string `json:"type"`
	Protocol string `json:"protocol"`
}
type UserCapacity struct {
	Blocks int64 `json:"blocks"`
	Bytes  int64 `json:"bytes"`
}
type FormFactor struct {
	ATAValue int    `json:"ata_value"`
	Name     string `json:"name"`
}
type SmartStatus struct {
	Passed bool `json:"passed"`
}
type Temperature struct {
	Current int `json:"current"`
}
type PowerOnTime struct {
	Hours int `json:"hours"`
}

// --- ATA SMART ---
type SATAVersion struct {
	Value  int    `json:"value"`
	String string `json:"string"`
}

type ATASmartAttrs struct {
	Revision int            `json:"revision"`
	Table    []ATASmartAttr `json:"table"`
}
type ATASmartAttr struct {
	ID         int         `json:"id"`
	Name       string      `json:"name"`
	Value      int         `json:"value"`
	Worst      int         `json:"worst"`
	Thresh     int         `json:"thresh"`
	WhenFailed string      `json:"when_failed"`
	Flags      ATAFlags    `json:"flags"`
	Raw        ATARawValue `json:"raw"`
}
type ATAFlags struct {
	Value      int    `json:"value"`
	String     string `json:"string"`
	Prefailure bool   `json:"prefailure"`
}
type ATARawValue struct {
	Value  int    `json:"value"`
	String string `json:"string"`
}

// --- NVMe SMART ---
type NVMeVersion struct {
	String string `json:"string"`
	Value  int    `json:"value"`
}

type NVMeHealth struct {
	CriticalWarning         int   `json:"critical_warning"`
	Temperature             int   `json:"temperature"`
	AvailableSpare          int   `json:"available_spare"`
	AvailableSpareThreshold int   `json:"available_spare_threshold"`
	PercentageUsed          int   `json:"percentage_used"`
	DataUnitsRead           int64 `json:"data_units_read"`
	DataUnitsWritten        int64 `json:"data_units_written"`
	HostReads               int64 `json:"host_reads"`
	HostWrites              int64 `json:"host_writes"`
	ControllerBusyTime      int64 `json:"controller_busy_time"`
	PowerCycles             int64 `json:"power_cycles"`
	PowerOnHours            int64 `json:"power_on_hours"`
	UnsafeShutdowns         int64 `json:"unsafe_shutdowns"`
	MediaErrors             int64 `json:"media_errors"`
	NumErrLogEntries        int64 `json:"num_err_log_entries"`
	WarningCompTempTime     int   `json:"warning_comp_temp_time"`
	CriticalCompTempTime    int   `json:"critical_comp_temp_time"`
}

// --- SCSI/SAS SMART ---
type ScsiErrorLog struct {
	Read   *ScsiErrorCounter `json:"read,omitempty"`
	Write  *ScsiErrorCounter `json:"write,omitempty"`
	Verify *ScsiErrorCounter `json:"verify,omitempty"`
}
type ScsiErrorCounter struct {
	ErrorsCorrectedByECCFast    int64           `json:"errors_corrected_by_eccfast,omitempty"`
	ErrorsCorrectedByECCDelayed int64           `json:"errors_corrected_by_eccdelayed,omitempty"`
	ErrorsCorrectedByReReads    int64           `json:"errors_corrected_by_rereads_rewrites,omitempty"`
	TotalErrorsCorrected        int64           `json:"total_errors_corrected,omitempty"`
	CorrectionAlgInvocations    int64           `json:"correction_algorithm_invocations,omitempty"`
	GigabytesProcessed          json.RawMessage `json:"gigabytes_processed,omitempty"`
	TotalUncorrectedErrors      int64           `json:"total_uncorrected_errors"`
}
type ScsiStartStop struct {
	SpecifiedCycleCountOverLife int `json:"specified_cycle_count_over_device_lifetime"`
	AccumulatedStartStopCycles  int `json:"accumulated_start_stop_cycles"`
	SpecifiedLoadUnloadCount    int `json:"specified_load_unload_count_over_device_lifetime"`
	AccumulatedLoadUnloadCycles int `json:"accumulated_load_unload_cycles"`
}
type ScsiTemperature struct {
	Current    int `json:"current"`
	DriveTripC int `json:"drive_trip,omitempty"`
}
