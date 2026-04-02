package product

import (
	"errors"
)

func (p *Product) Collect() error {
	errs := make([]error, 0, 5)

	os, err := collectOS()
	if err != nil {
		errs = append(errs, err)
	}

	bios, err := collectBIOS()
	if err != nil {
		errs = append(errs, err)
	}

	system, err := collectSystem()
	if err != nil {
		errs = append(errs, err)
	}

	baseboard, err := collectBaseBoard()
	if err != nil {
		errs = append(errs, err)
	}

	chassis, err := collectChassis()
	if err != nil {
		errs = append(errs, err)
	}

	p = &Product{
		OperatingSystem: *os,
		BIOS:            *bios,
		System:          *system,
		BaseBoard:       *baseboard,
		Chassis:         *chassis,
	}
	return errors.Join(errs...)
}

func (p *Product) Name() string {
	return "Product"
}

func (p *Product) JSON() {

}

func (p *Product) Sprintln() {}

func (p *Product) Lprintln() {}
