package sensehat

import (
	"fmt"
	"sync"
	"time"
)

// ST LPS25H Pressure & Temperature Sensor

type LPS25H struct {
	device      Device
	mut         sync.Mutex
	cached      time.Time
	temperature float64
	pressure    float64
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

func (s *LPS25H) Refresh(age time.Duration) error {
	s.mut.Lock()
	defer s.mut.Unlock()

	if time.Since(s.cached) < age {
		return nil
	}

	if err := s.device.SetAddress(lps25hAddress); err != nil {
		return fmt.Errorf("set device address: %w", err)
	}

	r := newDevReader(s.device)

	// Numeric constants from data sheet
	s.pressure = float64(r.signed(lps25hPressOutHReg, lps25hPressOutLReg, lps25HressOutXLReg)) / 4096
	s.temperature = float64(r.signed(lps25hTempOutHReg, lps25hTempOutLReg))/480 + 42.5

	if r.error != nil {
		return fmt.Errorf("read data: %w", r.error)
	}
	s.cached = time.Now()
	return nil
}

func (s *LPS25H) Temperature() float64 {
	s.mut.Lock()
	defer s.mut.Unlock()
	return s.temperature
}

func (s *LPS25H) Pressure() float64 {
	s.mut.Lock()
	defer s.mut.Unlock()
	return s.pressure
}
