package client

import "testing"

func TestPVESocketTier(t *testing.T) {
	cases := map[int]string{
		0:  "1", // nonsense clamps up to the smallest tier
		1:  "1",
		2:  "2",
		3:  "4", // 3 sockets needs the 4-tier
		4:  "4",
		5:  "8",
		8:  "8",
		16: "8", // above the top tier clamps to 8
	}
	for in, want := range cases {
		if got := PVESocketTier(in); got != want {
			t.Errorf("PVESocketTier(%d) = %q, want %q", in, got, want)
		}
	}
}
