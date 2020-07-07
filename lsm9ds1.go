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

func (s *LSM9DS1) AccelAngles() (a, b, c float64) {
	s.mut.Lock()
	defer s.mut.Unlock()
	return math.Atan2(float64(s.ay), float64(s.az)) / math.Pi * 180, math.Atan2(float64(s.ay), float64(s.az)) / math.Pi * 180, math.Atan2(float64(s.ax), float64(s.az)) / math.Pi * 180
}

func (s *LSM9DS1) MagneticAngles() (a, b, c float64) {
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

type AvgLSM9DS1 struct {
	*LSM9DS1
	intv       time.Duration
	mut        sync.Mutex
	accel      [][3]int16
	angles     [][3]float64
	ao, bo, co float64
}

func NewAvgLSM9DS1(total, intv time.Duration, lsm9ds1 *LSM9DS1, ao, bo, co float64) *AvgLSM9DS1 {
	size := int(total / intv)
	a := &AvgLSM9DS1{
		LSM9DS1: lsm9ds1,
		intv:    intv,
		accel:   make([][3]int16, 0, size),
		angles:  make([][3]float64, 0, size),
		ao:      ao,
		bo:      bo,
		co:      co,
	}
	go a.serve()
	return a
}

func (a *AvgLSM9DS1) serve() {
	for range time.NewTicker(a.intv).C {
		if err := a.LSM9DS1.Refresh(a.intv / 2); err != nil {
			continue
		}
		a.update()
	}
}

func (a *AvgLSM9DS1) update() {
	a.mut.Lock()
	defer a.mut.Unlock()
	x, y, z := a.LSM9DS1.Acceleration()
	p := angle(float64(z), float64(y), a.ao)
	r := angle(float64(z), float64(x), a.bo)
	w := angle(float64(y), float64(x), a.co)
	if len(a.accel) < cap(a.accel) {
		a.accel = append(a.accel, [3]int16{x, y, z})
		a.angles = append(a.angles, [3]float64{p, r, w})
	} else {
		copy(a.accel, a.accel[1:])
		copy(a.angles, a.angles[1:])
		a.accel[len(a.accel)-1] = [3]int16{x, y, z}
		a.angles[len(a.angles)-1] = [3]float64{p, r, w}
	}
}

func (a *AvgLSM9DS1) Acceleration() (x, y, z int16) {
	a.mut.Lock()
	defer a.mut.Unlock()
	if len(a.accel) == 0 {
		return 0, 0, 0
	}
	i := len(a.accel) / 2
	return a.accel[i][0], a.accel[i][1], a.accel[i][2]
}

func (a *AvgLSM9DS1) AccelAngles() (p, r, w float64) {
	a.mut.Lock()
	defer a.mut.Unlock()
	if len(a.angles) == 0 {
		return 0, 0, 0
	}
	i := len(a.angles) / 2
	return a.angles[i][0], a.angles[i][1], a.angles[i][2]
}

func (a *AvgLSM9DS1) Deviation() (p, r, w float64) {
	a.mut.Lock()
	defer a.mut.Unlock()
	if len(a.angles) == 0 {
		return 0, 0, 0
	}
	minp := a.angles[0][0]
	maxp := a.angles[0][0]
	minr := a.angles[0][1]
	maxr := a.angles[0][1]
	minw := a.angles[0][2]
	maxw := a.angles[0][2]
	for i := 1; i < len(a.angles); i++ {
		if a.angles[i][0] < minp {
			minp = a.angles[i][0]
		}
		if a.angles[i][0] > maxp {
			maxp = a.angles[i][0]
		}
		if a.angles[i][1] < minr {
			minr = a.angles[i][1]
		}
		if a.angles[i][1] > maxr {
			maxr = a.angles[i][1]
		}
		if a.angles[i][2] < minw {
			minw = a.angles[i][2]
		}
		if a.angles[i][2] > maxw {
			maxw = a.angles[i][2]
		}
	}
	return maxp - minp, maxr - minr, maxw - minw
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

func angle(y, x, o float64) float64 {
	return compass(y, x, o) - 180
}
