package omini

import "fmt"

// A Device is typically a *sysfs.I2cDevice (gobot.io/x/gobot/sysfs).
type Device interface {
	SetAddress(address int) error
	ReadByteData(reg uint8) (val uint8, err error)
	ReadWordData(reg uint8) (val uint16, err error)
	WriteByteData(reg, val uint8) error
}

type devReader struct {
	dev   Device
	error error
}

func newDevReader(dev Device) *devReader {
	return &devReader{dev: dev}
}

func (r *devReader) reset() {
	r.error = nil
}

func (r *devReader) read(regs ...uint8) ([]byte, error) {
	res := make([]byte, len(regs))

	for i := len(regs) - 1; i >= 0; i-- {
		val, err := r.dev.ReadByteData(regs[i])
		if err != nil {
			return nil, fmt.Errorf("read byte register: %w", err)
		}
		res[i] = val
	}
	return res, nil
}

func (r *devReader) signed(regs ...uint8) int {
	if r.error != nil {
		return 0
	}
	data, err := r.read(regs...)
	if err != nil {
		r.error = err
		return 0
	}
	return signed(data)
}

func (r *devReader) byte(reg uint8) int {
	if r.error != nil {
		return 0
	}
	val, err := r.dev.ReadByteData(reg)
	if err != nil {
		r.error = err
		return 0
	}
	return int(val)
}

func signed(data []byte) int {
	res := int(int8(data[0]))
	for _, val := range data[1:] {
		res <<= 8
		res |= int(val)
	}
	return res
}
