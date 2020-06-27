package sensehat

import (
	"fmt"
)

// ST HTS221 Humidity & Temperature Sensor

type HTS221 struct {
	h0rH    float64
	h1rH    float64
	t0degC  float64
	t1degC  float64
	h0t0Out float64
	h1t0Out float64
	t0Out   float64
	t1Out   float64
	tSlope  float64
	hSlope  float64
	device  Device
}

const (
	hts221Address     = 0x5f
	hts221CtrlReg1    = 0x20
	hts221InitData    = 0x85 // PD=1, ODR0=1, BDU=1
	hts221HumOutLReg  = 0x28
	hts221HumOutHReg  = 0x29
	hts221TempOutLReg = 0x2a
	hts221TempOutHReg = 0x2b
	h0rHx2Reg         = 0x30
	h1rHx2Reg         = 0x31
	t0degCx8Reg       = 0x32
	t1degCx8Reg       = 0x33
	t1t0msbReg        = 0x35
	h0t0OutRegL       = 0x36
	h0t0OutRegH       = 0x37
	h1t0OutRegL       = 0x3a
	h1t0OutRegH       = 0x3b
	t0OutRegL         = 0x3c
	t0OutRegH         = 0x3d
	t1OutRegL         = 0x3e
	t1OutRegH         = 0x3f
)

func NewHTS221(dev Device) (*HTS221, error) {
	// Initialize sensor

	if err := dev.SetAddress(hts221Address); err != nil {
		return nil, err
	}
	if err := dev.WriteByteData(hts221CtrlReg1, hts221InitData); err != nil {
		return nil, err
	}

	s := &HTS221{device: dev}

	// Read calibration data

	r := newDevReader(dev)

	s.h0rH = float64(r.byte(h0rHx2Reg)) / 2
	s.h1rH = float64(r.byte(h1rHx2Reg)) / 2
	s.t0degC = float64(r.byte(t0degCx8Reg))
	s.t1degC = float64(r.byte(t1degCx8Reg))
	t1t0msb := r.byte(t1t0msbReg)
	s.t0degC += float64((t1t0msb & 0x3) << 8)
	s.t1degC += float64((t1t0msb & 0xc) << 6)
	s.t0degC /= 8
	s.t1degC /= 8

	s.h0t0Out = float64(r.signed(h0t0OutRegH, h0t0OutRegL))
	s.h1t0Out = float64(r.signed(h1t0OutRegH, h1t0OutRegL))
	s.t0Out = float64(r.signed(t0OutRegH, t0OutRegL))
	s.t1Out = float64(r.signed(t1OutRegH, t1OutRegL))

	if r.error != nil {
		return nil, fmt.Errorf("read calibration data: %w", r.error)
	}

	s.tSlope = (s.t1degC - s.t0degC) / (s.t1Out - s.t0Out)
	s.hSlope = (s.h1rH - s.h0rH) / (s.h1t0Out - s.h0t0Out)

	return s, nil
}

func (s *HTS221) Data() (humidity, temperature float64, err error) {
	if err := s.device.SetAddress(hts221Address); err != nil {
		return 0, 0, fmt.Errorf("set device address: %w", err)
	}

	r := newDevReader(s.device)

	humidity = (float64(r.signed(hts221HumOutHReg, hts221HumOutLReg))-s.h0t0Out)*s.hSlope + s.h0rH
	temperature = (float64(r.signed(hts221TempOutHReg, hts221TempOutLReg))-s.t0Out)*s.tSlope + s.t0degC

	return humidity, temperature, fmt.Errorf("read data: %w", r.error)
}
