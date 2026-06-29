package subscription

import "testing"

func TestParseStatus(t *testing.T) {
	cases := []struct {
		in   string
		want Status
		ok   bool
	}{
		{"approved", Approved, true},
		{"PENDING", Pending, true},
		{" blocked ", Blocked, true},
		{"rejected", Rejected, true},
		{"nonsense", "", false},
		{"", "", false},
	}
	for _, c := range cases {
		got, err := ParseStatus(c.in)
		if c.ok && (err != nil || got != c.want) {
			t.Errorf("ParseStatus(%q) = (%q,%v), want (%q,nil)", c.in, got, err, c.want)
		}
		if !c.ok && err == nil {
			t.Errorf("ParseStatus(%q) expected error", c.in)
		}
	}
}

func TestStatusIsValid(t *testing.T) {
	if Status("WHATEVER").IsValid() {
		t.Error("unexpected valid status")
	}
	for _, s := range []Status{Approved, Pending, Blocked, Failed, Rejected, Registered} {
		if !s.IsValid() {
			t.Errorf("%s should be valid", s)
		}
	}
}
