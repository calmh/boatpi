package i2c

import "testing"

func TestSigned(t *testing.T) {
	cases := []struct {
		in  []byte
		out int
	}{
		{[]byte{1, 2, 3, 4}, 1<<24 + 2<<16 + 3<<8 + 4},
		{[]byte{0x7f, 0xff}, 0x7fff},
		{[]byte{0xff, 0xff}, -1},
	}

	for _, tc := range cases {
		if res := signed(tc.in); res != tc.out {
			t.Errorf("%d != expected %d for %v", res, tc.out, tc.in)
		}
	}
}
