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
	size       int
	intv       time.Duration
	mut        sync.Mutex
	x, y, z    []float64
	p, r, w    []float64
	xs, ys, zs float64
	ps, rs, ws float64
}

func NewAvgLSM9DS1(total, intv time.Duration, lsm9ds1 *LSM9DS1) *AvgLSM9DS1 {
	size := int(total / intv)
	a := &AvgLSM9DS1{
		lsm9ds1: lsm9ds1,
		size:    size,
		intv:    intv,
		x:       make([]float64, size),
		y:       make([]float64, size),
		z:       make([]float64, size),
		p:       make([]float64, size),
		r:       make([]float64, size),
		w:       make([]float64, size),
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
	a.xs -= a.x[a.size-1]
	a.ys -= a.y[a.size-1]
	a.zs -= a.z[a.size-1]
	a.ps -= a.p[a.size-1]
	a.rs -= a.r[a.size-1]
	a.ws -= a.w[a.size-1]
	copy(a.x[1:], a.x)
	copy(a.y[1:], a.y)
	copy(a.z[1:], a.z)
	copy(a.p[1:], a.p)
	copy(a.r[1:], a.r)
	copy(a.w[1:], a.w)
	a.x[0] = x
	a.y[0] = y
	a.z[0] = z
	a.p[0] = math.Atan(y/z) / math.Pi * 180
	a.r[0] = math.Atan(x/z) / math.Pi * 180
	a.w[0] = math.Atan(x/y) / math.Pi * 180
	a.xs += a.x[0]
	a.ys += a.y[0]
	a.zs += a.z[0]
	a.ps += a.p[0]
	a.rs += a.r[0]
	a.ws += a.w[0]
}

func (a *AvgLSM9DS1) Acceleration() (x, y, z float64) {
	a.mut.Lock()
	defer a.mut.Unlock()
	n := float64(a.size)
	return a.xs / n, a.ys / n, a.zs / n
}

func (a *AvgLSM9DS1) Angles() (p, r, w float64) {
	a.mut.Lock()
	defer a.mut.Unlock()
	n := float64(a.size)
	return a.ps / n, a.rs / n, a.ws / n
}

func (a *AvgLSM9DS1) Deviation() (p, r, w float64) {
	a.mut.Lock()
	defer a.mut.Unlock()
	minp := a.p[0]
	maxp := a.p[0]
	minr := a.r[0]
	maxr := a.r[0]
	minw := a.w[0]
	maxw := a.w[0]
	for i := 1; i < a.size; i++ {
		if a.p[i] < minp {
			minp = a.p[i]
		}
		if a.p[i] > maxp {
			maxp = a.p[i]
		}
		if a.r[i] < minr {
			minr = a.r[i]
		}
		if a.r[i] > maxr {
			maxr = a.r[i]
		}
		if a.w[i] < minw {
			minw = a.w[i]
		}
		if a.w[i] > maxw {
			maxw = a.w[i]
		}
	}
	return maxp - minp, maxr - minr, maxw - minw
}
