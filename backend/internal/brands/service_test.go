package brands

import "testing"

func TestNormalize(t *testing.T) {
	for in, want := range map[string]string{"  Knauf ": "Knauf", "Тест \t  Бренд": "Тест Бренд", "": ""} {
		if got := Normalize(in); got != want {
			t.Errorf("%q: %q", in, got)
		}
	}
	if _, err := name("x\u0007"); err == nil {
		t.Error("control characters must be refused")
	}
}
