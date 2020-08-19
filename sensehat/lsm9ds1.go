package sensehat

import (
	"fmt"
	"log"
	"math"
	"sync"
	"time"

	"github.com/calmh/boatpi/i2c"
)

// ST LSM9DS1 iNEMO inertial module, 3D magnetometer, 3D accelerometer, 3D
// gyroscope

type LSM9DS1 struct {
	device     i2c.Device
	mut        sync.Mutex
	cal        Calibration
	mo         float64
	cached     time.Time
	ax, ay, az int16
	mx, my, mz int16
}

type Point struct {
	X, Y, Z int16
}

type Calibration struct {
	Min Point
	Max Point
}

const (
	lsm9ds1AccelAddress    = 0x6a
	lsm9ds1AccelCtrlReg6XL = 0x20
	lsm9ds1AccelInitData   = 0b_001_00_000
	lsm9ds1AccelXOutXLReg  = 0x28
	lsm9ds1AccelYOutXLReg  = 0x2a
	lsm9ds1AccelZOutXLReg  = 0x2c

	lsm9ds1MagnAddress  = 0x1c
	lsm9ds1MagnXOutLReg = 0x28
	lsm9ds1MagnYOutLReg = 0x2a
	lsm9ds1MagnZOutLReg = 0x2c
)

var magnInitData = [][2]byte{
	{0x20, 0b_1001_0000}, // CTRL_REG1_M
	{0x21, 0b_0000_1100}, // CTRL_REG2_M
	{0x22, 0b_0000_0000}, // CTRL_REG3_M
}

func NewLSM9DS1(dev i2c.Device, magnOffs float64, cal Calibration) (*LSM9DS1, error) {
	// Initialize sensors

	if err := dev.SetAddress(lsm9ds1AccelAddress); err != nil {
		return nil, fmt.Errorf("set device address: %w", err)
	}
	if err := dev.WriteByteData(lsm9ds1AccelCtrlReg6XL, lsm9ds1AccelInitData); err != nil {
		return nil, fmt.Errorf("write control register 6_XL: %w", err)
	}
	if err := dev.SetAddress(lsm9ds1MagnAddress); err != nil {
		return nil, fmt.Errorf("set device address: %w", err)
	}
	for _, line := range magnInitData {
		if err := dev.WriteByteData(line[0], line[1]); err != nil {
			log.Printf("write control register 0x%02x->0x%02x: %v", line[1], line[0], err)
		}
	}

	return &LSM9DS1{device: dev, cal: cal, mo: magnOffs}, nil
}

func (s *LSM9DS1) Refresh(age time.Duration) error {
	s.mut.Lock()
	defer s.mut.Unlock()

	if time.Since(s.cached) < age {
		return nil
	}

	r := i2c.NewReader(s.device)

	if err := s.device.SetAddress(lsm9ds1AccelAddress); err != nil {
		return fmt.Errorf("set device address: %w", err)
	}

	s.ax = int16(r.Signed(lsm9ds1AccelXOutXLReg+1, lsm9ds1AccelXOutXLReg))
	s.ay = int16(r.Signed(lsm9ds1AccelYOutXLReg+1, lsm9ds1AccelYOutXLReg))
	s.az = int16(r.Signed(lsm9ds1AccelZOutXLReg+1, lsm9ds1AccelZOutXLReg))
	if err := r.Error(); err != nil {
		return fmt.Errorf("read data: %w", err)
	}

	if err := s.device.SetAddress(lsm9ds1MagnAddress); err != nil {
		return fmt.Errorf("set device address: %w", err)
	}

	s.mx = int16(r.Signed(lsm9ds1MagnXOutLReg+1, lsm9ds1MagnXOutLReg))
	s.my = int16(r.Signed(lsm9ds1MagnYOutLReg+1, lsm9ds1MagnYOutLReg))
	s.mz = int16(r.Signed(lsm9ds1MagnZOutLReg+1, lsm9ds1MagnZOutLReg))
	if err := r.Error(); err != nil {
		return fmt.Errorf("read data: %w", err)
	}

	s.updateCalibration(s.mx, s.my, s.mz)
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

func (s *LSM9DS1) AccelerationAngles() (xy, xz, yz float64) {
	s.mut.Lock()
	defer s.mut.Unlock()
	xy = angle(float64(s.ay), float64(s.ax))
	xz = angle(float64(s.az), float64(s.ax))
	yz = angle(float64(s.az), float64(s.ay))
	return xy, xz, yz
}

func (s *LSM9DS1) MagneticField() (x, y, z int16) {
	s.mut.Lock()
	defer s.mut.Unlock()
	return s.mx, s.my, s.mz
}

func (s *LSM9DS1) Compass() (xy, xz, yz float64) {
	s.mut.Lock()
	defer s.mut.Unlock()
	x := float64(s.mx - (s.cal.Max.X+s.cal.Min.X)/2)
	y := float64(s.my - (s.cal.Max.Y+s.cal.Min.Y)/2)
	z := float64(s.mz - (s.cal.Max.Z+s.cal.Min.Z)/2)
	return compass(y, x, s.mo), compass(z, x, s.mo), compass(z, y, s.mo)
}

func (s *LSM9DS1) updateCalibration(x, y, z int16) {
	if s.cal.Max.X == 0 || x > s.cal.Max.X {
		s.cal.Max.X = x
	}
	if s.cal.Min.X == 0 || x < s.cal.Min.X {
		s.cal.Min.X = x
	}
	if s.cal.Max.Y == 0 || y > s.cal.Max.Y {
		s.cal.Max.Y = y
	}
	if s.cal.Min.Y == 0 || y < s.cal.Min.Y {
		s.cal.Min.Y = y
	}
	if s.cal.Max.Z == 0 || z > s.cal.Max.Z {
		s.cal.Max.Z = z
	}
	if s.cal.Min.Z == 0 || z < s.cal.Min.Z {
		s.cal.Min.Z = z
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

func angle(y, x float64) float64 {
	v := math.Atan2(y, x) / math.Pi * 180
	for v > 180 {
		v -= 360
	}
	for v < -180 {
		v += 360
	}
	return v
}
