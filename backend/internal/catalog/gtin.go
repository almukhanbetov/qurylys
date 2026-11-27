package catalog

import "strings"

// NormalizeGTIN accepts GTIN-8, -12, -13 and -14 with a correct check digit and
// returns the 14-digit form. It proves only that the number is well formed; the
// source of the number is checked by a moderator.
func NormalizeGTIN(raw string) (string, bool) {
	v := strings.NewReplacer(" ", "", "-", "").Replace(strings.TrimSpace(raw))
	switch len(v) {
	case 8, 12, 13, 14:
	default:
		return "", false
	}
	v = strings.Repeat("0", 14-len(v)) + v
	sum, zero := 0, true
	for i := 0; i < len(v); i++ {
		d := int(v[i]) - '0'
		if d < 0 || d > 9 {
			return "", false
		}
		zero = zero && d == 0
		if i%2 == 0 {
			d *= 3
		}
		sum += d
	}
	return v, sum%10 == 0 && !zero
}
