package pilot

import (
	"math"
	"sort"
)

// Numeric helpers with exact, explicit semantics (NaN handling, saturating float-to-int
// conversions, Euclidean remainder, stable sorting).

// fmin returns the smaller value; if one argument is NaN, the other is returned.
func fmin(a, b float64) float64 {
	if math.IsNaN(a) {
		return b
	}
	if math.IsNaN(b) {
		return a
	}
	if a < b {
		return a
	}
	if b < a {
		return b
	}
	// equal (incl. +0/-0): either is fine; keep a unless a is +0 and b is -0
	if a == 0 && math.Signbit(b) {
		return b
	}
	return a
}

// fmax returns the larger value; if one argument is NaN, the other is returned.
func fmax(a, b float64) float64 {
	if math.IsNaN(a) {
		return b
	}
	if math.IsNaN(b) {
		return a
	}
	if a > b {
		return a
	}
	if b > a {
		return b
	}
	if a == 0 && !math.Signbit(b) {
		return b
	}
	return a
}

// fclamp clamps x to [lo, hi] (NaN stays NaN); it panics when lo > hi or a bound is NaN.
func fclamp(x, lo, hi float64) float64 {
	if !(lo <= hi) {
		panic("fclamp: lo > hi or NaN bound")
	}
	if x < lo {
		x = lo
	}
	if x > hi {
		x = hi
	}
	return x
}

// iclamp clamps an int64.
func iclamp(x, lo, hi int64) int64 {
	if lo > hi {
		panic("iclamp: lo > hi")
	}
	if x < lo {
		return lo
	}
	if x > hi {
		return hi
	}
	return x
}

// uclamp clamps an int.
func uclamp(x, lo, hi int) int {
	if lo > hi {
		panic("uclamp: lo > hi")
	}
	if x < lo {
		return lo
	}
	if x > hi {
		return hi
	}
	return x
}

// remEuclid is the non-negative (Euclidean) remainder.
func remEuclid(a, b float64) float64 {
	r := math.Mod(a, b)
	if r < 0 {
		return r + math.Abs(b)
	}
	return r
}

// signum: 1 for +0 and positives, -1 for -0 and negatives, NaN for NaN.
func signum(x float64) float64 {
	if math.IsNaN(x) {
		return math.NaN()
	}
	if math.Signbit(x) {
		return -1
	}
	return 1
}

// powi raises x to an integer power by repeated squaring.
func powi(x float64, n int32) float64 {
	neg := n < 0
	e := int64(n)
	if neg {
		e = -e
	}
	r := 1.0
	b := x
	for e > 0 {
		if e&1 == 1 {
			r *= b
		}
		b *= b
		e >>= 1
	}
	if neg {
		return 1 / r
	}
	return r
}

// f2i64 converts to i64, saturating at the bounds (NaN -> 0).
func f2i64(x float64) int64 {
	switch {
	case math.IsNaN(x):
		return 0
	case x >= 9.223372036854775807e18:
		return math.MaxInt64
	case x <= -9.223372036854775808e18:
		return math.MinInt64
	}
	return int64(x)
}

// f2i32 converts to i32, saturating at the bounds (NaN -> 0).
func f2i32(x float64) int32 {
	switch {
	case math.IsNaN(x):
		return 0
	case x >= 2147483647:
		return math.MaxInt32
	case x <= -2147483648:
		return math.MinInt32
	}
	return int32(x)
}

// f2u is `x as usize` (saturating at 0, NaN -> 0).
func f2u(x float64) int {
	switch {
	case math.IsNaN(x) || x <= 0:
		return 0
	case x >= 9.2e18:
		return math.MaxInt
	}
	return int(x)
}

// f2u32 converts to u32, saturating at the bounds (NaN -> 0).
func f2u32(x float64) uint32 {
	switch {
	case math.IsNaN(x) || x <= 0:
		return 0
	case x >= 4294967295:
		return math.MaxUint32
	}
	return uint32(x)
}

// f2u64 converts to u64, saturating at the bounds (NaN -> 0).
func f2u64(x float64) uint64 {
	switch {
	case math.IsNaN(x) || x <= 0:
		return 0
	case x >= 1.8446744073709552e19:
		return math.MaxUint64
	}
	return uint64(x)
}

// f2u8 converts to u8, saturating at the bounds (NaN -> 0).
func f2u8(x float64) uint8 {
	switch {
	case math.IsNaN(x) || x <= 0:
		return 0
	case x >= 255:
		return 255
	}
	return uint8(x)
}

// partialCmp is a three-way float comparison that panics on NaN.
func partialCmp(a, b float64) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	case a == b:
		return 0
	}
	panic("partial_cmp on NaN")
}

// partialCmpOr is a three-way float comparison that treats NaN as equal.
func partialCmpOr(a, b float64) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	}
	return 0
}

// totalCmp orders floats by their IEEE total order.
func totalCmp(a, b float64) int {
	ia := int64(math.Float64bits(a))
	ib := int64(math.Float64bits(b))
	ia ^= int64(uint64(ia>>63) >> 1)
	ib ^= int64(uint64(ib>>63) >> 1)
	switch {
	case ia < ib:
		return -1
	case ia > ib:
		return 1
	}
	return 0
}

// sortStable sorts s stably with a three-way comparison.
func sortStable[T any](s []T, cmp func(a, b *T) int) {
	sort.SliceStable(s, func(i, j int) bool { return cmp(&s[i], &s[j]) < 0 })
}

// isFinite reports whether x is neither NaN nor infinite.
func isFinite(x float64) bool { return !math.IsNaN(x) && !math.IsInf(x, 0) }

// ptr returns a pointer to a copy of v.
func ptr[T any](v T) *T { return &v }
