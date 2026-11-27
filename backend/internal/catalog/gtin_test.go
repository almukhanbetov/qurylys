package catalog

import "testing"

func TestNormalizeGTIN(t *testing.T) {
	for in, want := range map[string]string{
		"2000000000756": "02000000000756", " 2000-0000 00756 ": "02000000000756",
		"96385074": "00000096385074", "036000291452": "00036000291452", "02000000000756": "02000000000756",
	} {
		if got, ok := NormalizeGTIN(in); !ok || got != want {
			t.Errorf("%q: %q %v", in, got, ok)
		}
	}
	for _, in := range []string{"", "2000000000755", "200000000075", "123", "20000000007561", "200000000075x", "00000000", "٢٠٠٠٠٠٠٠", "200000000075600"} {
		if got, ok := NormalizeGTIN(in); ok {
			t.Errorf("%q accepted as %q", in, got)
		}
	}
}
