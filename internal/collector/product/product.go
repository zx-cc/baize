// Package product collects server platform identification data from SMBIOS
// firmware tables and the running operating system.
//
// The five sub-collectors map to the following SMBIOS table types:
//   - OS        — parsed from /etc/os-release (not SMBIOS)
//   - BIOS      — SMBIOS type 0
//   - System    — SMBIOS type 1
//   - BaseBoard — SMBIOS type 2
//   - Chassis   — SMBIOS type 3
package product

import (
	"errors"

	"github.com/zx-cc/baize/pkg/utils"
)

// New returns an initialised Product collector.
func New() *Product {
	return &Product{}
}

// Collect runs all five sub-collectors concurrently and joins any errors.
func (p *Product) Collect() error {
	errs := make([]error, 0, 5)

	os, err := collectOS()
	if err != nil {
		errs = append(errs, err)
	}
	p.OS = os

	bios, err := collectBIOS()
	if err != nil {
		errs = append(errs, err)
	}
	p.BIOS = bios

	system, err := collectSystem()
	if err != nil {
		errs = append(errs, err)
	}
	p.System = system

	baseboard, err := collectBaseBoard()
	if err != nil {
		errs = append(errs, err)
	}
	p.BaseBoard = baseboard

	chassis, err := collectChassis()
	if err != nil {
		errs = append(errs, err)
	}
	p.Chassis = chassis

	return errors.Join(errs...)
}

// Name returns the module identifier used for routing by the collector manager.
func (p *Product) Name() string {
	return "PRODUCT"
}

// Jprintln serialises the collected product data to JSON and writes it to stdout.
func (p *Product) Jprintln() error {
	return utils.JSONPrintln(p)
}

// Sprintln prints a brief product summary to stdout.
func (p *Product) Sprintln() {}

// Lprintln prints a detailed product report to stdout.
func (p *Product) Lprintln() {}
