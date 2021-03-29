package sensorbug

import "testing"

var manufacturerData = []byte{133, 0, 2, 0, 60, 100, 3, 67, 249, 0}

func TestParseHeader(t *testing.T) {
	var h header
	h.Unmarshal(manufacturerData[:4])
	t.Logf("%#v", h)

	var s static
	s.Unmarshal(manufacturerData[4:])
	t.Logf("%#v", s)

	var d dynamic
	d.Unmarshal(manufacturerData[7:])
	t.Logf("%#v", d)
}
