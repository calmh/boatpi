package omini

import (
	"fmt"
	"sync"

	"github.com/calmh/boatpi/i2c"
)

type Omini struct {
	dev i2c.Device
	mut sync.Mutex
}

const (
	ominiAddress       = 0x29
	ominiChannelARegHi = 1
	ominiChannelBRegHi = 3
	ominiChannelCRegHi = 5
)

func New(dev i2c.Device) *Omini {
	return &Omini{dev: dev}
}

func (s *Omini) Voltages() (a, b, c float64, err error) {
	s.mut.Lock()
	defer s.mut.Unlock()

	if err := s.dev.SetAddress(ominiAddress); err != nil {
		return 0, 0, 0, fmt.Errorf("set device address: %w", err)
	}

	r := i2c.NewReader(s.dev)
	a = float64(r.Signed(ominiChannelARegHi)) + float64(r.Signed(ominiChannelARegHi+1))/100
	b = float64(r.Signed(ominiChannelBRegHi)) + float64(r.Signed(ominiChannelBRegHi+1))/100
	c = float64(r.Signed(ominiChannelCRegHi)) + float64(r.Signed(ominiChannelCRegHi+1))/100
	err = r.Error()
	return
}
