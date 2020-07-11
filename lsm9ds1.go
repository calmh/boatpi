package sensehat

import (
	"fmt"
	"math"
	"sync"
	"time"
)

// ST LSM9DS1 iNEMO inertial module, 3D magnetometer, 3D accelerometer, 3D
// gyroscope

type LSM9DS1 struct {
	device     Device
	mut        sync.Mutex
	cal        Calibration
	mo         float64
	cached     time.Time
	ax, ay, az int16
	mx, my, mz int16
}

type Calibration struct {
	MinX, MaxX int16
	MinY, MaxY int16
	MinZ, MaxZ int16
}

const (
	lsm9ds1AccelAddress    = 0x6a
	lsm9ds1AccelCtrlReg6XL = 0x20
	lsm9ds1AccelInitData   = 0b_001_00_000
	lsm9ds1AccelXOutXLReg  = 0x28
	lsm9ds1AccelYOutXLReg  = 0x2a
	lsm9ds1AccelZOutXLReg  = 0x2c

	lsm9ds1MagnAddress   = 0x1c
	lsm9ds1MagnCtrlReg3M = 0x22
	lsm9ds1MagnInitData  = 0b_00_0_000_00
	lsm9ds1MagnXOutLReg  = 0x28
	lsm9ds1MagnYOutLReg  = 0x2a
	lsm9ds1MagnZOutLReg  = 0x2c
)

func NewLSM9DS1(dev Device, magnOffs float64, cal Calibration) (*LSM9DS1, error) {
	// Initialize sensors

	if err := dev.SetAddress(lsm9ds1AccelAddress); err != nil {
		return nil, fmt.Errorf("set device address: %w", err)
	}
	if err := dev.WriteByteData(lsm9ds1AccelCtrlReg6XL, lsm9ds1AccelInitData); err != nil {
		return nil, fmt.Errorf("write control register: %w", err)
	}
	if err := dev.SetAddress(lsm9ds1MagnAddress); err != nil {
		return nil, fmt.Errorf("set device address: %w", err)
	}
	if err := dev.WriteByteData(lsm9ds1MagnCtrlReg3M, lsm9ds1MagnInitData); err != nil {
		return nil, fmt.Errorf("write control register: %w", err)
	}

	return &LSM9DS1{device: dev, cal: cal, mo: magnOffs}, nil
}

func (s *LSM9DS1) Refresh(age time.Duration) error {
	s.mut.Lock()
	defer s.mut.Unlock()

	if time.Since(s.cached) < age {
		return nil
	}

	r := newDevReader(s.device)

	if err := s.device.SetAddress(lsm9ds1AccelAddress); err != nil {
		return fmt.Errorf("set device address: %w", err)
	}

	s.ax = int16(r.signed(lsm9ds1AccelXOutXLReg+1, lsm9ds1AccelXOutXLReg))
	s.ay = int16(r.signed(lsm9ds1AccelYOutXLReg+1, lsm9ds1AccelYOutXLReg))
	s.az = int16(r.signed(lsm9ds1AccelZOutXLReg+1, lsm9ds1AccelZOutXLReg))

	if err := s.device.SetAddress(lsm9ds1MagnAddress); err != nil {
		return fmt.Errorf("set device address: %w", err)
	}

	s.mx = int16(r.signed(lsm9ds1MagnXOutLReg+1, lsm9ds1MagnXOutLReg))
	s.my = int16(r.signed(lsm9ds1MagnYOutLReg+1, lsm9ds1MagnYOutLReg))
	s.mz = int16(r.signed(lsm9ds1MagnZOutLReg+1, lsm9ds1MagnZOutLReg))
	s.updateCalibration(s.mx, s.my, s.mz)

	if r.error != nil {
		return fmt.Errorf("read data: %w", r.error)
	}
	s.cached = time.Now()
	return nil
}

func (s *LSM9DS1) Calibration() Calibration {
	s.mut.Lock()
	defer s.mut.Unlock()
	return s.cal
}

func (s *LSM9DS1) Acceleration() (x, y, z int16) {
	s.mut.Lock()
	defer s.mut.Unlock()
	return s.ax, s.ay, s.az
}

func (s *LSM9DS1) MagneticField() (x, y, z int16) {
	s.mut.Lock()
	defer s.mut.Unlock()
	return s.mx, s.my, s.mz
}

func (s *LSM9DS1) Compass() (a, b, c float64) {
	s.mut.Lock()
	defer s.mut.Unlock()
	x := float64(s.mx - (s.cal.MaxX+s.cal.MinX)/2)
	y := float64(s.my - (s.cal.MaxY+s.cal.MinY)/2)
	z := float64(s.mz - (s.cal.MaxZ+s.cal.MinZ)/2)
	return compass(y, x, s.mo), compass(y, z, s.mo), compass(x, z, s.mo)
}

func (s *LSM9DS1) updateCalibration(x, y, z int16) {
	if x > s.cal.MaxX {
		s.cal.MaxX = x
	}
	if x < s.cal.MinX {
		s.cal.MinX = x
	}
	if y > s.cal.MaxY {
		s.cal.MaxY = y
	}
	if y < s.cal.MinY {
		s.cal.MinY = y
	}
	if z > s.cal.MaxZ {
		s.cal.MaxZ = z
	}
	if z < s.cal.MinZ {
		s.cal.MinZ = z
	}
}

func compass(y, x, o float64) float64 {
	v := math.Atan2(y, x)/math.Pi*180 + o
	for v > 360 {
		v -= 360
	}
	for v < 0 {
		v += 360
	}
	return v
}
