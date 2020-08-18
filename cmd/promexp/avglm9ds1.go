package main

import (
	"log"
	"math"
	"sync"
	"time"

	"github.com/calmh/boatpi/sensehat"
)

type AvgLSM9DS1 struct {
	*sensehat.LSM9DS1
	intv   time.Duration
	mut    sync.Mutex
	accel  [][3]int16
	angles [][3]float64
}

func NewAvgLSM9DS1(total, intv time.Duration, lsm9ds1 *sensehat.LSM9DS1) *AvgLSM9DS1 {
	size := int(total / intv)
	a := &AvgLSM9DS1{
		LSM9DS1: lsm9ds1,
		intv:    intv,
		accel:   make([][3]int16, 0, size),
		angles:  make([][3]float64, 0, size),
	}
	go a.serve()
	return a
}

func (a *AvgLSM9DS1) serve() {
	for range time.NewTicker(a.intv).C {
		if err := a.LSM9DS1.Refresh(a.intv / 2); err != nil {
			log.Println("refresh llsm9ds1:", err)
			continue
		}
		a.update()
	}
}

func (a *AvgLSM9DS1) update() {
	a.mut.Lock()
	defer a.mut.Unlock()
	x, y, z := a.LSM9DS1.Acceleration()
	xy := angle(float64(y), float64(x))
	xz := angle(float64(z), float64(x))
	yz := angle(float64(z), float64(y))
	if len(a.accel) < cap(a.accel) {
		a.accel = append(a.accel, [3]int16{x, y, z})
		a.angles = append(a.angles, [3]float64{xy, xz, yz})
	} else {
		copy(a.accel, a.accel[1:])
		copy(a.angles, a.angles[1:])
		a.accel[len(a.accel)-1] = [3]int16{x, y, z}
		a.angles[len(a.angles)-1] = [3]float64{xy, xz, yz}
	}
}

func (a *AvgLSM9DS1) AccelAngles() (xy, xz, yz float64) {
	a.mut.Lock()
	defer a.mut.Unlock()
	if len(a.angles) == 0 {
		return 0, 0, 0
	}
	i := len(a.angles) / 2
	return a.angles[i][0], a.angles[i][1], a.angles[i][2]
}

func (a *AvgLSM9DS1) Deviation() (xy, xz, yz float64) {
	a.mut.Lock()
	defer a.mut.Unlock()
	if len(a.angles) == 0 {
		return 0, 0, 0
	}
	minxy := a.angles[0][0]
	maxxy := a.angles[0][0]
	minxz := a.angles[0][1]
	maxxz := a.angles[0][1]
	minyz := a.angles[0][2]
	maxyz := a.angles[0][2]
	for i := 1; i < len(a.angles); i++ {
		if a.angles[i][0] < minxy {
			minxy = a.angles[i][0]
		}
		if a.angles[i][0] > maxxy {
			maxxy = a.angles[i][0]
		}
		if a.angles[i][1] < minxz {
			minxz = a.angles[i][1]
		}
		if a.angles[i][1] > maxxz {
			maxxz = a.angles[i][1]
		}
		if a.angles[i][2] < minyz {
			minyz = a.angles[i][2]
		}
		if a.angles[i][2] > maxyz {
			maxyz = a.angles[i][2]
		}
	}
	return maxxy - minxy, maxxz - minxz, maxyz - minyz
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
