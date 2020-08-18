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
	err = r.Error()
	return
}
