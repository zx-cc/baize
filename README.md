# 白泽 (Baize) — Hardware Information Collector

> A lightweight, extensible server hardware information collector written in Go.
> Named after the mythological creature 白泽 (*Baize*) — said to know all things in the world.

---

## Overview

**Baize** collects comprehensive hardware and firmware information from Linux
servers without any external agent or daemon.  It reads data directly from the
kernel (sysfs, procfs), firmware (SMBIOS/DMI), and vendor management CLIs
(storcli, hpssacli, arcconf, ipmitool, smartctl) and presents the results as
structured JSON or formatted terminal output.

```
╔══════════════════════════════════════════════════╗
║  白泽 (Baize) — Hardware Information Collector   ║
╚══════════════════════════════════════════════════╝
```

---

## Features

| Module   | Data sources                                            | Key information collected                                    |
|----------|---------------------------------------------------------|--------------------------------------------------------------|
| `cpu`    | `lscpu`, SMBIOS type-4, sysfs hwmon                    | Architecture, topology, cache, frequency, per-core temperature |
| `memory` | `/proc/meminfo`, SMBIOS type-17, EDAC sysfs            | DIMM inventory, ECC errors, usage counters, health diagnosis |
| `network`| `/sys/class/net`, PCI, `/proc/net/bonding`             | Logical/physical NICs, PCIe info, bond/LACP configuration    |
| `raid`   | PCI scan, storcli / hpssacli / arcconf / mdadm, smartctl | RAID controllers, logical/physical drives, SMART, NVMe     |
| `gpu`    | `/sys/class/drm`, PCI scan                             | Graphics card model, PCIe link, on-board vs. discrete flag   |
| `ipmi`   | `ipmitool`                                             | BMC info, sensors, PSU status, System Event Log              |
| `product`| SMBIOS types 0–3, `/etc/os-release`                    | OS, BIOS, system, baseboard, chassis information             |

---

## Requirements

- **OS**: Linux (kernel ≥ 4.15 recommended)
- **Go**: 1.24.2 or later
- **Privileges**: Most modules require root / CAP_SYS_RAWIO access
- **Optional tools** (only needed for the corresponding module):
  - `storcli`   — Broadcom/LSI RAID controllers
  - `hpssacli`  — HPE Smart Array controllers
  - `arcconf`   — Microchip/Adaptec controllers
  - `mdadm`     — Intel VROC / software RAID
  - `ipmitool`  — IPMI/BMC data
  - `smartctl`  — SMART data for all storage devices

---

## Getting Started

### Build

```bash
git clone https://github.com/zx-cc/baize.git
cd baize
go build -o baize ./cmd/cli
```

### Run

```bash
# Collect all modules and print formatted output
sudo ./baize

# Collect a specific module
sudo ./baize -m cpu
sudo ./baize -m memory
sudo ./baize -m network
sudo ./baize -m raid
sudo ./baize -m gpu
sudo ./baize -m ipmi
sudo ./baize -m product

# Output as JSON (suitable for piping to jq or a log aggregator)
sudo ./baize -j
sudo ./baize -m cpu -j

# Print detailed (long) output instead of the default brief summary
sudo ./baize -d
```

### CLI Flags

| Flag | Default | Description                                        |
|------|---------|----------------------------------------------------|
| `-m` | `all`   | Module name to collect (`all` runs every module)   |
| `-j` | `false` | Output results as JSON                             |
| `-d` | `false` | Print detailed view instead of brief summary       |

---

## Project Structure

```
baize/
├── cmd/
│   └── cli/
│       └── main.go              # CLI entry point (flag parsing, banner, timing)
├── internal/
│   └── collector/
│       ├── cpu/                 # CPU topology, cache, per-core temperature
│       ├── gpu/                 # GPU discovery via DRM and PCI
│       ├── ipmi/                # IPMI sensors, BMC, PSU, SEL (concurrent)
│       ├── memory/              # DIMM inventory, meminfo, EDAC error counters
│       ├── network/             # NICs, bonds, PCIe details
│       ├── pci/                 # PCI device enumeration and pci.ids lookup
│       ├── product/             # Server identity (BIOS, system, chassis, OS)
│       ├── raid/                # RAID controllers, NVMe, physical/logical drives
│       ├── smart/               # Vendor-neutral smartctl wrapper (SATA/SAS/NVMe)
│       └── smbios/              # Pure-Go SMBIOS table parser (types 0,1,2,3,4,17)
├── pkg/
│   ├── logger/                  # Structured logging helpers
│   ├── paths/                   # Centralised sysfs/procfs path constants
│   ├── shell/                   # Shell command execution helpers
│   └── utils/                   # Scanners, size formatting, sysfs helpers
├── go.mod
└── README.md
```

---

## Module Details

### CPU (`-m cpu`)

Collects processor information from three sources and correlates them:

1. **`lscpu`** — architecture, byte order, cache hierarchy, feature flags
2. **SMBIOS type-4** — per-socket designation, core/thread counts, voltage, upgrade type
3. **sysfs hwmon** — real-time per-core temperature readings

### Memory (`-m memory`)

| Source              | Data                                        |
|---------------------|---------------------------------------------|
| `/proc/meminfo`     | Total, free, available, swap, huge pages    |
| SMBIOS type-17      | DIMM size, manufacturer, part number, speed, rank |
| `/sys/bus/edac`     | ECC correctable/uncorrectable error counters |

A health diagnosis is generated automatically:
- Slot count mismatch between SMBIOS and EDAC
- OS-visible memory significantly less than SMBIOS physical size
- Odd DIMM count (asymmetric configuration)

### Network (`-m network`)

Three interface types are collected:

- **NetInterface** — all logical interfaces from `/sys/class/net` with speed, state, and MAC
- **PhyInterface** — physical NICs resolved through PCI, with driver and firmware info
- **BondInterface** — bond mode, LACP rate, hash policy, and per-slave status from `/proc/net/bonding`

### RAID / Storage (`-m raid`)

PCI class IDs `0x0104` / `0x0107` trigger RAID controller collection;
class ID `0x0108` triggers NVMe collection.

**Supported RAID controllers:**

| Vendor            | PCI ID | Tool        |
|-------------------|--------|-------------|
| Broadcom / LSI    | `1000` | `storcli`   |
| Microchip/Adaptec | `9005` | `arcconf`   |
| HPE Smart Array   | `103C` | `hpssacli`  |
| Intel VROC        | `8086` | `mdadm`     |

For each controller the collector gathers: controller card details, physical
drives (with SMART), logical drives (RAID level, state), enclosures, and
battery/CacheVault units.

**NVMe drives** are discovered via sysfs (`/sys/bus/pci/devices/<bus>/nvme/`)
and SMART data is collected with `smartctl -d nvme`.

### GPU (`-m gpu`)

Discovers graphics cards using two strategies (tried in order):

1. DRM enumeration — `/sys/class/drm/card*`
2. PCI class scan — all devices with class ID `0x03`

Each card is tagged as on-board or discrete based on a vendor:device ID
allow-list (Matrox, ASPEED, HiSilicon management controllers are marked
on-board).

### IPMI (`-m ipmi`)

Runs four `ipmitool` sub-commands concurrently:

| Sub-task | Command                 | Data                                |
|----------|-------------------------|-------------------------------------|
| BMC      | `bmc info` / `lan print`| Firmware version, IP, MAC           |
| Sensors  | `sensor`                | Temperature, voltage, fan, current  |
| Power    | `sdr` / `dcmi`          | PSU presence/status, total power    |
| SEL      | `sel elist`             | Filtered system event log entries   |

Post-collection diagnosis flags critical/warning SEL events, sensor alarms,
and PSU failures.

### Product (`-m product`)

| Sub-collector | SMBIOS type | Data                                     |
|---------------|-------------|------------------------------------------|
| BIOS          | Type 0      | Vendor, version, release date            |
| System        | Type 1      | Manufacturer, product name, UUID, serial |
| BaseBoard     | Type 2      | Manufacturer, product, serial, asset tag |
| Chassis       | Type 3      | Type, manufacturer, serial               |
| OS            | —           | Distribution, version, kernel            |

---

## Output Examples

### JSON output (`-j`)

```bash
sudo ./baize -m cpu -j | jq '.model_name, .socket\(s\)'
```

```json
{
  "model_name": "Intel(R) Xeon(R) Gold 6154 CPU @ 3.00GHz",
  "vendor": "Intel",
  "architecture": "x86_64",
  "socket(s)": "2",
  "cores_per_socket": "18",
  "l3_cache": "24.8 MiB",
  ...
}
```

### Terminal output

```
╔══════════════════════════════════════════════════╗
║  白泽 (Baize) — Hardware Information Collector   ║
╚══════════════════════════════════════════════════╝

── CPU INFO ──────────────────────────────────────
  Model Name   Intel(R) Xeon(R) Gold 6154 @ 3.00GHz
  Vendor       Intel
  Sockets      2
  Cores/Socket 18
  Threads/Core 2
  L3 Cache     24.8 MiB

── Collection completed in 3.21s ──
```

---

## Architecture

```
┌─────────────────────────────────────────────────────┐
│                    cmd/cli/main.go                   │
│  flag parsing → manager.NewManager → print / JSON   │
└───────────────────────┬─────────────────────────────┘
                        │ dispatches by module name
        ┌───────────────┼──────────────────┐
        ▼               ▼                  ▼
  collector/cpu   collector/raid    collector/ipmi  …
        │               │
        ▼               ▼
  pkg/shell       internal/pci     internal/smart
  pkg/utils       internal/smbios
```

Each collector implements a small interface:

```go
type Collector interface {
    Name()     string
    Collect()  error
    Jprintln() error  // JSON output
    Sprintln()        // brief terminal output
    Lprintln()        // detailed terminal output
}
```

---

## Contributing

1. Fork the repository and create a feature branch.
2. Follow the existing code style (Go standard formatting, English comments).
3. Add or update tests for any new behaviour.
4. Submit a pull request with a clear description of the change.

---

## License

See [LICENSE](LICENSE) for details.