package omini

import (
	"fmt"
	"sync"
)

type Omini struct {
	dev Device
	mut sync.Mutex
}

const (
	ominiAddress       = 0x29
	ominiChannelARegHi = 1
	ominiChannelBRegHi = 3
	ominiChannelCRegHi = 5
)

func New(dev Device) *Omini {
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

	r := newDevReader(s.dev)
	v := float64(r.signed(regHi)) + float64(r.signed(regHi+1))/100
	if r.error != nil {
		return 0, fmt.Errorf("read data: %w", r.error)
	}

	return v, nil
}
