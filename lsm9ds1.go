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
	device  Device
	mut     sync.Mutex
	cached  time.Time
	x, y, z float64
}

const (
	lsm9ds1Address        = 0x6a
	lsm9ds1CtrlReg6XL     = 0x20
	lsm9ds1InitData       = 0b_001_00_000
	lsm9ds1AccelXOutXLReg = 0x28
	lsm9ds1AccelYOutXLReg = 0x2a
	lsm9ds1AccelZOutXLReg = 0x2c
)

func NewLSM9DS1(dev Device) (*LSM9DS1, error) {
	// Initialize sensor

	if err := dev.SetAddress(lsm9ds1Address); err != nil {
		return nil, fmt.Errorf("set device address: %w", err)
	}
	if err := dev.WriteByteData(lsm9ds1CtrlReg6XL, lsm9ds1InitData); err != nil {
		return nil, fmt.Errorf("write control register: %w", err)
	}

	return &LSM9DS1{device: dev}, nil
}

func (s *LSM9DS1) Refresh(age time.Duration) error {
	s.mut.Lock()
	defer s.mut.Unlock()

	if time.Since(s.cached) < age {
		return nil
	}

	if err := s.device.SetAddress(lsm9ds1Address); err != nil {
		return fmt.Errorf("set device address: %w", err)
	}

	r := newDevReader(s.device)

	s.x = float64(r.signed(lsm9ds1AccelXOutXLReg+1, lsm9ds1AccelXOutXLReg))
	s.y = float64(r.signed(lsm9ds1AccelYOutXLReg+1, lsm9ds1AccelYOutXLReg))
	s.z = float64(r.signed(lsm9ds1AccelZOutXLReg+1, lsm9ds1AccelZOutXLReg))

	if r.error != nil {
		return fmt.Errorf("read data: %w", r.error)
	}
	s.cached = time.Now()
	return nil
}

func (s *LSM9DS1) Acceleration() (x, y, z float64) {
	s.mut.Lock()
	defer s.mut.Unlock()
	return s.x, s.y, s.z
}

func (s *LSM9DS1) Angles() (roll, pitch, yaw float64) {
	s.mut.Lock()
	defer s.mut.Unlock()
	return math.Atan(s.x/s.z) / math.Pi * 180, math.Atan(s.y/s.z) / math.Pi * 180, math.Atan(s.x/s.y) / math.Pi * 180
}

type AvgLSM9DS1 struct {
	lsm9ds1    *LSM9DS1
	intv       time.Duration
	mut        sync.Mutex
	accel      [][3]float64
	angles     [][3]float64
	po, ro, wo float64
}

func NewAvgLSM9DS1(total, intv time.Duration, lsm9ds1 *LSM9DS1, po, ro, wo float64) *AvgLSM9DS1 {
	size := int(total / intv)
	a := &AvgLSM9DS1{
		lsm9ds1: lsm9ds1,
		intv:    intv,
		accel:   make([][3]float64, 0, size),
		angles:  make([][3]float64, 0, size),
		po:      po,
		ro:      ro,
		wo:      wo,
	}
	go a.serve()
	return a
}

func (a *AvgLSM9DS1) serve() {
	for range time.NewTicker(a.intv).C {
		if err := a.lsm9ds1.Refresh(a.intv / 2); err != nil {
			continue
		}
		a.update()
	}
}

func (a *AvgLSM9DS1) update() {
	a.mut.Lock()
	defer a.mut.Unlock()
	x, y, z := a.lsm9ds1.Acceleration()
	p := math.Atan(z/y)/math.Pi*180 + a.po
	r := math.Atan(z/x)/math.Pi*180 + a.ro
	w := math.Atan(y/x)/math.Pi*180 + a.wo
	if len(a.accel) < cap(a.accel) {
		a.accel = append(a.accel, [3]float64{x, y, z})
		a.angles = append(a.angles, [3]float64{p, r, w})
	} else {
		copy(a.accel, a.accel[1:])
		copy(a.angles, a.angles[1:])
		a.accel[len(a.accel)-1] = [3]float64{x, y, z}
		a.angles[len(a.angles)-1] = [3]float64{p, r, w}
	}
}

func (a *AvgLSM9DS1) Acceleration() (x, y, z float64) {
	a.mut.Lock()
	defer a.mut.Unlock()
	if len(a.accel) == 0 {
		return 0, 0, 0
	}
	i := len(a.accel) / 2
	return a.accel[i][0], a.accel[i][1], a.accel[i][2]
}

func (a *AvgLSM9DS1) Angles() (p, r, w float64) {
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
