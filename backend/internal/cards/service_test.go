package cards

import "testing"

func TestSameNumbers(t *testing.T) {
	for _, c := range []struct {
		a, b string
		want bool
	}{
		{"Монтажная пена 750 мл", "Монтажная пена, 750 мл", true},
		{"Монтажная пена 750 мл", "Монтажная пена 500 мл", false},
		{"Саморез 4,2×75", "Саморез 4.2 × 75 мм", true},
		{"Саморез 4,2×75", "Саморез 4,2×90", false},
		{"Цемент М500 50 кг", "Цемент 50 кг М500", true},
		{"Цемент М500 50 кг", "Цемент М400 50 кг", false},
		{"Клей", "Клей плиточный", true},
		{"Клей", "Клей 2", false},
	} {
		if got := sameNumbers(c.a, c.b); got != c.want {
			t.Errorf("%q vs %q: %v", c.a, c.b, got)
		}
	}
}
