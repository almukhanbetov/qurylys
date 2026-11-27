package quantity

import "testing"

func q(t *testing.T, s string) Q {
	t.Helper()
	v, ok := Parse(s)
	if !ok {
		t.Fatalf("parse %q", s)
	}
	return v
}

func TestParse(t *testing.T) {
	for in, want := range map[string]string{"12": "12", "0.6": "0.6", "1.250": "1.25", "0.001": "0.001", "99999999999.999": "99999999999.999", "007": "7"} {
		if got := q(t, in).String(); got != want {
			t.Errorf("%s: %s", in, got)
		}
	}
	for _, in := range []string{"", "-1", "+1", "1e3", "1.2345", "1,5", ".5", "5.", "NaN", "0x1", "999999999999", " 1", "١"} {
		if v, ok := Parse(in); ok {
			t.Errorf("%q accepted as %v", in, v)
		}
	}
}

// The same cases are checked in SQL (TestMoneyRuleInSQL) and in the browser code
// (frontend/src/lib/quantity.test.ts).
func TestTotal(t *testing.T) {
	for _, c := range []struct {
		price int64
		qty   string
		want  int64
	}{
		{150000, "12", 1800000}, {115000, "12", 1380000}, {115000, "60", 6900000}, {8437, "500", 4218500},
		{83333, "0.6", 50000}, {1, "0.5", 1}, {1, "1", 1}, {1, "0.499", 0}, {3, "0.5", 2}, {9007199254740991, "1", 9007199254740991},
	} {
		if got, ok := Total(c.price, q(t, c.qty)); !ok || got != c.want {
			t.Errorf("%d × %s = %d %v, want %d", c.price, c.qty, got, ok, c.want)
		}
	}
	if _, ok := Total(MaxMoney, q(t, "99999999999.999")); ok {
		t.Error("overflow accepted")
	}
}

func TestSaleRule(t *testing.T) {
	for _, c := range []struct {
		unit, mode, pack, min, step string
		ok                          bool
	}{
		{"piece", "piece", "12", "1", "1", true}, {"piece", "pack", "12", "60", "12", true}, {"piece", "pack", "12", "50", "12", false},
		{"piece", "pack", "12", "60", "10", false}, {"piece", "pack", "0", "60", "12", false}, {"piece", "piece", "0", "1.5", "1", false},
		{"piece", "piece", "0", "2", "2", false}, {"piece", "pack", "1.5", "3", "1.5", false}, {"piece", "pack", "12", "60", "24", false}, {"piece", "pack", "12", "48", "24", true},
		{"kg", "piece", "0", "500", "0.5", true}, {"kg", "piece", "0", "0.7", "0.5", false}, {"m3", "pack", "0.6", "1.2", "0.6", true},
		{"m3", "pack", "0.6", "1", "0.6", false}, {"m3", "piece", "0", "1.5", "0.1", true}, {"kg", "box", "1", "1", "1", false},
		{"kg", "piece", "0", "0", "1", false}, {"kg", "piece", "0", "1", "0", false},
	} {
		if got := SaleRuleOK(c.unit, c.mode, q(t, c.pack), q(t, c.min), q(t, c.step)); got != c.ok {
			t.Errorf("%+v: %v", c, got)
		}
	}
}
