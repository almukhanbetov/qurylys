// Package quantity holds exact decimal quantities (up to 3 fraction digits, like
// NUMERIC(14,3)) and money arithmetic without floating point.
package quantity

import (
	"math/big"
	"regexp"
	"strconv"
	"strings"
)

// Q is a number of thousandths of a base unit.
type Q int64

const One Q = 1000

// MaxMoney is the largest amount that JavaScript also holds exactly.
const MaxMoney int64 = 1<<53 - 1

var decimalRe = regexp.MustCompile(`^[0-9]{1,11}(\.[0-9]{1,3})?$`)

// Parse accepts "12", "0.6" and "1.250"; no signs, exponents or rounding.
func Parse(s string) (Q, bool) {
	if !decimalRe.MatchString(s) {
		return 0, false
	}
	whole, frac, _ := strings.Cut(s, ".")
	n, err := strconv.ParseInt(whole+(frac + "000")[:3], 10, 64)
	return Q(n), err == nil
}

func (q Q) String() string {
	s := strconv.FormatInt(int64(q)/1000, 10)
	if f := int64(q) % 1000; f != 0 {
		s += "." + strings.TrimRight((strconv.FormatInt(1000+f, 10))[1:], "0")
	}
	return s
}

func (q Q) Whole() bool { return q%One == 0 }

// MultipleOf is exact; other must be positive.
func (q Q) MultipleOf(other Q) bool { return other > 0 && q%other == 0 }

// Total is price × quantity in tiyn, rounded half up once. False on overflow.
func Total(priceTiyn int64, q Q) (int64, bool) {
	t := new(big.Int).Mul(big.NewInt(priceTiyn), big.NewInt(int64(q)))
	t.Add(t, big.NewInt(500)).Quo(t, big.NewInt(1000))
	if !t.IsInt64() || t.Int64() > MaxMoney {
		return 0, false
	}
	return t.Int64(), true
}

var discrete = map[string]bool{"piece": true, "bag": true, "pack": true, "roll": true, "sheet": true, "pallet": true}

// Discrete reports whether the unit is sold only in whole numbers.
func Discrete(unit string) bool { return discrete[unit] }

// SaleRuleOK mirrors sale_rule_ok in migration 00010. pack is 0 when absent.
func SaleRuleOK(unit, mode string, pack, min, step Q) bool {
	if min <= 0 || step <= 0 || !min.MultipleOf(step) {
		return false
	}
	if Discrete(unit) && (!min.Whole() || !step.Whole() || !pack.Whole()) {
		return false
	}
	switch mode {
	case "pack":
		return step.MultipleOf(pack)
	case "piece":
		return !Discrete(unit) || step == One
	}
	return false
}
