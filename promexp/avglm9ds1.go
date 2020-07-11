package main

import (
	"math"
	"sync"
	"time"

	"github.com/kastelo-labs/sensehat"
)

type AvgLSM9DS1 struct {
	*sensehat.LSM9DS1
	intv       time.Duration
	mut        sync.Mutex
	accel      [][3]int16
	angles     [][3]float64
	ao, bo, co float64
}

func NewAvgLSM9DS1(total, intv time.Duration, lsm9ds1 *sensehat.LSM9DS1, ao, bo, co float64) *AvgLSM9DS1 {
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

func angle(y, x, o float64) float64 {
	v := math.Atan2(y, x)/math.Pi*180 + o
	for v > 180 {
		v -= 360
	}
	for v < -180 {
		v += 360
	}
	return v
}
