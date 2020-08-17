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

func (s *Omini) ChannelAVoltage() (float64, error) {
	return s.voltage(ominiChannelARegHi)
}

func (s *Omini) ChannelBVoltage() (float64, error) {
	return s.voltage(ominiChannelBRegHi)
}

func (s *Omini) ChannelCVoltage() (float64, error) {
	return s.voltage(ominiChannelCRegHi)
}

func (s *Omini) voltage(regHi uint8) (float64, error) {
	s.mut.Lock()
	defer s.mut.Unlock()

	if err := s.dev.SetAddress(ominiAddress); err != nil {
		return 0, fmt.Errorf("set device address: %w", err)
	}

	r := i2c.NewReader(s.dev)
	v := float64(r.Signed(regHi)) + float64(r.Signed(regHi+1))/100
	if err := r.Error(); err != nil {
		return 0, fmt.Errorf("read data: %w", err)
	}

	return v, nil
}
