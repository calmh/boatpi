package sensehat

import (
	"fmt"
)

// ST LPS25H Pressure & Temperature Sensor

type LPS25H struct {
	device Device
}

const (
	lps25hAddress      = 0x5c
	lps25hCtrlReg1     = 0x20
	lps25hInitData     = 0x94 // PD=1, ODR0=1, BDU=1
	lps25HressOutXLReg = 0x28
	lps25hPressOutLReg = 0x29
	lps25hPressOutHReg = 0x2a
	lps25hTempOutLReg  = 0x2b
	lps25hTempOutHReg  = 0x2c
)

func NewLPS25H(dev Device) (*LPS25H, error) {
	// Initialize sensor

	if err := dev.SetAddress(lps25hAddress); err != nil {
		return nil, fmt.Errorf("set device address: %w", err)
	}
	if err := dev.WriteByteData(lps25hCtrlReg1, lps25hInitData); err != nil {
		return nil, fmt.Errorf("write control register: %w", err)
	}

	return &LPS25H{device: dev}, nil
}

func (s *LPS25H) Data() (pressure, temperature float64, err error) {
	if err := s.device.SetAddress(lps25hAddress); err != nil {
		return 0, 0, fmt.Errorf("set device address: %w", err)
	}

	r := newDevReader(s.device)

	// Numeric constants from data sheet
	pressure = float64(r.signed(lps25hPressOutHReg, lps25hPressOutLReg, lps25HressOutXLReg)) / 4096
	temperature = float64(r.signed(lps25hTempOutHReg, lps25hTempOutLReg))/480 + 42.5

	if r.error != nil {
		return 0, 0, fmt.Errorf("read data: %w", r.error)
	}
	return pressure, temperature, nil
}
