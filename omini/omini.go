package omini

import (
	"fmt"
	"log"
	"math"
	"sort"
	"sync"

	"github.com/calmh/boatpi/i2c"
)

const medianFilterSize = 51

type Omini struct {
	dev        i2c.Device
	mut        sync.Mutex
	a, b, c    float64
	pa, pb, pc floatset
}

const (
	ominiAddress       = 0x29
	ominiChannelARegHi = 1
	ominiChannelBRegHi = 3
	ominiChannelCRegHi = 5
)

func New(dev i2c.Device) *Omini {
	return &Omini{
		dev: dev,
		pa:  make(floatset, 0, medianFilterSize),
		pb:  make(floatset, 0, medianFilterSize),
		pc:  make(floatset, 0, medianFilterSize),
	}
}

func (s *Omini) Voltages() (a, b, c float64, err error) {
	s.mut.Lock()
	defer s.mut.Unlock()

	if err := s.dev.SetAddress(ominiAddress); err != nil {
		return 0, 0, 0, fmt.Errorf("set device address: %w", err)
	}

	r := i2c.NewReader(s.dev)

	a, b, c = s.voltages(r)
	s.pa = s.pa.append(a)
	s.pb = s.pb.append(b)
	s.pc = s.pc.append(c)

	if !s.pa.filled() || math.Abs(a-s.pa.median()) < 0.5 {
		s.a = a
	} else {
		log.Printf("Discarding a=%v (median %v)", a, s.pa.median())
	}
	if !s.pb.filled() || math.Abs(b-s.pb.median()) < 0.5 {
		s.b = b
	} else {
		log.Printf("Discarding b=%v (median %v)", b, s.pb.median())
	}
	if !s.pc.filled() || math.Abs(c-s.pc.median()) < 0.5 {
		s.c = c
	} else {
		log.Printf("Discarding c=%v (median %v)", c, s.pc.median())
	}

	return s.a, s.b, s.c, r.Error()
}

func (s *Omini) voltages(r *i2c.Reader) (a, b, c float64) {
	bs, err := r.Read(
		ominiChannelARegHi, ominiChannelARegHi+1,
		ominiChannelBRegHi, ominiChannelBRegHi+1,
		ominiChannelCRegHi, ominiChannelCRegHi+1,
	)
	if err == nil {
		// We sometimes seem to get the high bit set spuriously
		a = float64(bs[0]&^128) + float64(bs[1]&^128)/100
		b = float64(bs[2]&^128) + float64(bs[3]&^128)/100
		c = float64(bs[4]&^128) + float64(bs[5]&^128)/100
	}
	return
}

type floatset []float64

func (f floatset) append(v float64) floatset {
	if len(f) == cap(f) {
		copy(f, f[1:])
		f = f[:len(f)-1]
	}
	f = append(f, v)
	return f
}

func (f floatset) filled() bool {
	return len(f) == cap(f)
}

func (f floatset) median() float64 {
	c := make([]float64, len(f))
	copy(c, f)
	sort.Float64s(c)
	return c[len(c)/2]
}
