package pilot

import "math"

type V2 [2]float64
type V3 [3]float64

func wrap(a float64) float64 {
	if !isFinite(a) {
		return 0.0
	}
	x := remEuclid(a+math.Pi, 2.0*math.Pi) - math.Pi
	if x <= -math.Pi {
		return x + 2.0*math.Pi
	}
	return x
}

func adiff(a, b float64) float64 {
	return wrap(a - b)
}

func fin(x, d float64) float64 {
	if isFinite(x) {
		return x
	}
	return d
}

func add(a, b V3) V3 {
	return V3{a[0] + b[0], a[1] + b[1], a[2] + b[2]}
}

func sub(a, b V3) V3 {
	return V3{a[0] - b[0], a[1] - b[1], a[2] - b[2]}
}

func scale(a V3, k float64) V3 {
	return V3{a[0] * k, a[1] * k, a[2] * k}
}

func dot(a, b V3) float64 {
	return a[0]*b[0] + a[1]*b[1] + a[2]*b[2]
}

func len_(a V3) float64 {
	return math.Sqrt(dot(a, a))
}

func dist(a, b V3) float64 {
	return len_(sub(a, b))
}

func dist2(a, b V3) float64 {
	return math.Hypot(a[0]-b[0], a[1]-b[1])
}

func dist2v(a V3, b V2) float64 {
	return math.Hypot(a[0]-b[0], a[1]-b[1])
}

func up(a V3, h float64) V3 {
	return V3{a[0], a[1], a[2] + h}
}

func unit2(x, y float64) V2 {
	l := math.Hypot(x, y)
	if l > 1e-9 && isFinite(l) {
		return V2{x / l, y / l}
	}
	return V2{0.0, 0.0}
}

func aimDir(yaw, pitch float64) V3 {
	return V3{math.Cos(pitch) * math.Cos(yaw), math.Cos(pitch) * math.Sin(yaw), math.Sin(pitch)}
}

func dirToAim(d V3) V2 {
	h := math.Hypot(d[0], d[1])
	if h < 1e-9 && math.Abs(d[2]) < 1e-9 {
		return V2{0.0, 0.0}
	}
	return V2{math.Atan2(d[1], d[0]), math.Atan2(d[2], h)}
}

func slew(cur, cmd V2, dyaw, dpitch float64) V2 {
	ey := adiff(cmd[0], cur[0])
	ep := cmd[1] - cur[1]
	return V2{wrap(cur[0] + fclamp(ey, -dyaw, dyaw)), cur[1] + fclamp(ep, -dpitch, dpitch)}
}

// Rng is a splitmix64 generator.
type Rng struct {
	f0 uint64
}

func rngNew(seed uint64) Rng {
	return Rng{f0: seed ^ 0x9E3779B97F4A7C15}
}

func (r *Rng) nextU64() uint64 {
	r.f0 = r.f0 + 0x9E3779B97F4A7C15
	z := r.f0
	z = (z ^ (z >> 30)) * 0xBF58476D1CE4E5B9
	z = (z ^ (z >> 27)) * 0x94D049BB133111EB
	return z ^ (z >> 31)
}

func (r *Rng) f64() float64 {
	return float64(r.nextU64()>>11) / float64(uint64(1)<<53)
}

func (r *Rng) range_(a, b float64) float64 {
	return a + (b-a)*r.f64()
}

// interceptTime is the smallest t > 0 with |p + v t| = s t.
func interceptTime(p, v V3, s float64) *float64 {
	if !(s > 0.0) {
		return nil
	}
	a := dot(v, v) - s*s
	b := 2.0 * dot(p, v)
	c := dot(p, p)
	if math.Abs(a) < 1e-9 {
		if b < -1e-12 {
			t := -c / b
			if t > 0.0 && isFinite(t) {
				return ptr(t)
			}
			return nil
		}
		return nil
	}
	disc := b*b - 4.0*a*c
	if disc < 0.0 || !isFinite(disc) {
		return nil
	}
	sq := math.Sqrt(disc)
	t1 := (-b - sq) / (2.0 * a)
	t2 := (-b + sq) / (2.0 * a)
	var lo, hi float64
	if t1 < t2 {
		lo, hi = t1, t2
	} else {
		lo, hi = t2, t1
	}
	if lo > 1e-9 {
		return ptr(lo)
	} else if hi > 1e-9 {
		return ptr(hi)
	}
	return nil
}

// lead returns the aim point (relative to the shooter) and flight time for a constant-speed shot.
func lead(p, v, acc V3, s, latency, accGain, accT float64) *struct {
	f0 V3
	f1 float64
} {
	p0 := add(p, scale(v, latency))
	tp := interceptTime(p0, v, s)
	if tp == nil {
		return nil
	}
	t := *tp
	pt := add(p0, scale(v, t))
	if accGain > 0.0 {
		for i := 0; i < 3; i++ {
			ta := fmin(t, accT)
			pt = add(add(p0, scale(v, t)), scale(acc, 0.5*accGain*ta*ta))
			t = len_(pt) / s
		}
	}
	if isFinite(t) && isFinite(pt[0]) && isFinite(pt[1]) && isFinite(pt[2]) {
		return &struct {
			f0 V3
			f1 float64
		}{pt, t}
	}
	return nil
}

// mortarSolution returns launch pitch and flight time to hit a point at horizontal distance d and height dz.
func mortarSolution(d, dz, v, g float64, high bool) *struct {
	f0 float64
	f1 float64
} {
	if !(d > 1e-6) || !(v > 0.0) || !(g > 0.0) {
		return nil
	}
	v2 := v * v
	disc := v2*v2 - g*(g*d*d+2.0*dz*v2)
	if disc < 0.0 || !isFinite(disc) {
		return nil
	}
	sq := math.Sqrt(disc)
	var tan float64
	if high {
		tan = (v2 + sq) / (g * d)
	} else {
		tan = (v2 - sq) / (g * d)
	}
	pitch := math.Atan(tan)
	c := math.Cos(pitch)
	if c < 1e-6 {
		return nil
	}
	return &struct {
		f0 float64
		f1 float64
	}{pitch, d / (v * c)}
}

// closestXy is the closest horizontal approach of p + v t for t in [t0, t1]: (t, horizontal distance, dz).
func closestXy(p, v V3, t0, t1 float64) struct {
	f0 float64
	f1 float64
	f2 float64
} {
	vv := v[0]*v[0] + v[1]*v[1]
	var t float64
	if vv > 1e-12 {
		t = -(p[0]*v[0] + p[1]*v[1]) / vv
	} else {
		t = t0
	}
	if !isFinite(t) {
		t = t0
	}
	t = fclamp(t, t0, fmax(t1, t0))
	x := p[0] + v[0]*t
	y := p[1] + v[1]*t
	return struct {
		f0 float64
		f1 float64
		f2 float64
	}{t, math.Hypot(x, y), p[2] + v[2]*t}
}

func frac(a, b float64) float64 {
	if b > 1e-9 && isFinite(a) {
		return fclamp(a/b, 0.0, 1.0)
	}
	return 1.0
}
