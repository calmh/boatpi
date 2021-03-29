package sensorbug

import (
	"encoding/binary"
	"errors"
)

type header struct {
	// MSDLen        byte
	// MSDAdType     byte
	BlueRadiosCID uint16
	MajorPID      byte
	MinorPID      byte
}

func (h *header) Unmarshal(data []byte) error {
	if len(data) < 4 {
		return errors.New("too short")
	}
	// h.MSDLen = data[0]
	// h.MSDAdType = data[1]
	h.BlueRadiosCID = binary.LittleEndian.Uint16(data[0:])
	h.MajorPID = data[2]
	h.MinorPID = data[3]
	return nil
}

type static struct {
	TemplateID byte
	Battery    byte
	Config     byte
}

func (s *static) Unmarshal(data []byte) error {
	if len(data) < 3 {
		return errors.New("too short")
	}
	s.TemplateID = data[0]
	s.Battery = data[1]
	s.Config = data[2]
	return nil
}

type dynamic struct {
	Type  byte
	Alert bool
	Data  bool
	Temp  float64
}

func (d *dynamic) Unmarshal(data []byte) error {
	d.Type = data[0] & 0x37
	d.Alert = data[0]&(1<<7) != 0
	d.Data = data[0]&(1<<6) != 0
	if d.Data {
		d.Temp = float64(binary.LittleEndian.Uint16(data[1:])) * 0.0625
	}
	return nil
}
