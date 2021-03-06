// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package value

import "math/big"

func asin(v Value) Value {
	return evalFloatFunc(v, floatAsin)
}

func acos(v Value) Value {
	return evalFloatFunc(v, floatAcos)
}

func atan(v Value) Value {
	return evalFloatFunc(v, floatAtan)
}

// floatAsin computes asin(x) using the formula asin(x) = atan(x/sqrt(1-x²)).
func floatAsin(x *big.Float) *big.Float {
	// The asin Taylor series converges very slowly near ±1, but our
	// atan implementation converges well for all values, so we use
	// the formula above to compute asin. But be careful when |x|=1.
	if x.Cmp(floatOne) == 0 {
		z := newF().Set(floatPi)
		return z.Quo(z, floatTwo)
	}
	if x.Cmp(floatMinusOne) == 0 {
		z := newF().Set(floatPi)
		z.Quo(z, floatTwo)
		return z.Neg(z)
	}
	z := newF()
	z.Mul(x, x)
	z.Sub(floatOne, z)
	z = floatSqrt(z)
	z.Quo(x, z)
	return floatAtan(z)
}

// floatAcos computes acos(x) as π/2 - asin(x).
func floatAcos(x *big.Float) *big.Float {
	// acos(x) = π/2 - asin(x)
	z := newF().Set(floatPi)
	z.Quo(z, newF().SetInt64(2))
	return z.Sub(z, floatAsin(x))
}

// floatAtan computes atan(x) using a Taylor series. There are two series,
// one for |x| < 1 and one for larger values.
func floatAtan(x *big.Float) *big.Float {
	// atan(-x) == -atan(x). Do this up top to simplify the Euler crossover calculation.
	if x.Sign() < 0 {
		z := newF().Set(x)
		z = floatAtan(z.Neg(z))
		return z.Neg(z)
	}

	// The series converge very slowly near 1. atan 1.00001 takes over a million
	// iterations at the default precision. But there is hope, an Euler identity:
	//	atan(x) = atan(y) + atan((x-y)/(1+xy))
	// Note that y is a free variable. If x is near 1, we can use this formula
	// to push the computation to values that converge faster. Because
	//	tan(π/8) = √2 - 1, or equivalently atan(√2 - 1) == π/8
	// we choose y = √2 - 1 and then we only need to calculate one atan:
	//	atan(x) = π/8 + atan((x-y)/(1+xy))
	// Where do we cross over? This version converges significantly faster
	// even at 0.5, but we must be careful that (x-y)/(1+xy) never approaches 1.
	// At x = 0.5, (x-y)/(1+xy) is 0.07; at x=1 it is 0.414214; at x=1.5 it is
	// 0.66, which is as big as we dare go. With 256 bits of precision and a
	// crossover at 0.5, here are the number of iterations done by
	//	atan .1*iota 20
	// 0.1 39, 0.2 55, 0.3 73, 0.4 96, 0.5 126, 0.6 47, 0.7 59, 0.8 71, 0.9 85, 1.0 99, 1.1 116, 1.2 38, 1.3 44, 1.4 50, 1.5 213, 1.6 183, 1.7 163, 1.8 147, 1.9 135, 2.0 125
	tmp := newF().Set(floatOne)
	tmp.Sub(tmp, x)
	tmp.Abs(tmp)
	if tmp.Cmp(newF().SetFloat64(0.5)) < 0 {
		z := newF().Set(floatPi)
		z.Quo(z, newF().SetInt64(8))
		y := floatSqrt(floatTwo)
		y.Sub(y, floatOne)
		num := newF().Set(x)
		num.Sub(num, y)
		den := newF().Set(x)
		den = den.Mul(den, y)
		den = den.Add(den, floatOne)
		z = z.Add(z, floatAtan(num.Quo(num, den)))
		return z
	}

	if x.Cmp(floatOne) > 0 {
		return floatAtanLarge(x)
	}

	// This is the series for small values |x| <  1.
	// asin(x) = x - x³/3 + x⁵/5 - x⁷/7 + ...
	// First term to compute in loop will be x

	n := newF().Set(floatOne)
	two := newF().SetInt64(2)
	term := newF()
	xN := newF().Set(x)
	xSquared := newF().Set(x)
	xSquared.Mul(x, x)
	z := newF()
	plus := true

	loop := newLoop("atan", x, 4)
	// n goes up by two each loop.
	for {
		term.Set(xN)
		term.Quo(term, n)
		if plus {
			z.Add(z, term)
		} else {
			z.Sub(z, term)
		}
		plus = !plus

		if loop.terminate(z) {
			break
		}
		// Advance.
		n.Add(n, two)
		// xN *= x², becoming x**(n+2).
		xN.Mul(xN, xSquared)
	}

	return z
}

// floatAtanLarge computes atan(x)  for large x using a Taylor series.
// x is known to be > 1.
func floatAtanLarge(x *big.Float) *big.Float {
	// This is the series for larger values |x| >=  1.
	// For x > 0, atan(x) = +π/2 - 1/x + 1/3x³ -1/5x⁵ + 1/7x⁷ - ...
	// First term to compute in loop will be -1/x

	n := newF().Set(floatOne)
	term := newF()
	xN := newF().Set(x)
	xSquared := newF().Set(x)
	xSquared.Mul(x, x)
	z := newF().Set(floatPi)
	z.Quo(z, floatTwo)
	plus := false

	loop := newLoop("atan", x, 4)
	// n goes up by two each loop.
	for {
		term.Set(xN)
		term.Mul(term, n)
		term.Quo(floatOne, term)
		if plus {
			z.Add(z, term)
		} else {
			z.Sub(z, term)
		}
		plus = !plus

		if loop.terminate(z) {
			break
		}
		// Advance.
		n.Add(n, floatTwo)
		// xN *= x², becoming x**(n+2).
		xN.Mul(xN, xSquared)
	}

	return z
}
