// Package ink implements sampled quadratic ink for MyGo's native GPU painter.
package ink

import "math"

type Point struct{ X, Y, Radius float32 }
type Tool uint8

const (
	Pen Tool = iota
	Highlighter
	Pencil
)

type Stroke struct {
	Points      []Point
	Smooth      bool       // Points are pointer samples; connect curves at shared midpoints.
	Color       [4]float32 // straight sRGB, alpha; each component in [0,1]
	Translation [2]float32
	Tool        Tool
	Erasures    [][]Point // Eraser sweeps applied only to this existing stroke.
	Stamps      []Stamp   // optional explicit pencil stamps; nil generates them
}

// Stamp preserves the source's two-texel layout, without quantizing floats:
// (x,y,opacity,angle), (width,height,atlasIndex,padding). Signed size flips UVs.
type Stamp [2][4]float32

type Segment struct {
	P      [3]Point   // normalized by the height of the padded AABB
	Bounds [4]float32 // model x,y,width,height
	Phase  [2]float32
	Mode   int // 0 curve, 1 line, 2 dot
	A, B   [2]float32
	KK, KX float32
}

func length(x, y float32) float32     { return float32(math.Hypot(float64(x), float64(y))) }
func Quad(a, b, c, t float32) float32 { u := 1 - t; return u*u*a + 2*u*t*b + t*t*c }
func clamp(x, a, b float32) float32   { return min(max(x, a), b) }

// phaseOffset projects the endpoint center onto the circle-envelope line.
// The chord midpoint is this projection: dXY·(X-C) = -r*dr.
// If the envelope misses the circle, use its outward radial point.
func phaseOffset(p, q Point) float32 {
	dx, dy, dr := q.X-p.X, q.Y-p.Y, q.Radius-p.Radius
	l := length(dx, dy)
	if l < 1e-9 {
		return 0
	}
	if dr <= 0 {
		return 0
	} // projection points toward control; discard by direction test
	return min(p.Radius*dr/l, p.Radius)
}

// Segments uses sliding windows, with explicit one/two point behavior.
// pixelSize is one physical pixel measured in model units (1/(zoom*DPR)).
func Segments(points []Point, translation [2]float32, pixelSize float32) []Segment {
	if len(points) == 0 {
		return nil
	}
	return appendSegments(make([]Segment, 0, max(1, len(points)-2)), points, translation, pixelSize)
}

func appendSegments(out []Segment, points []Point, translation [2]float32, pixelSize float32) []Segment {
	if len(points) == 0 {
		return out
	}
	if pixelSize <= 0 || math.IsNaN(float64(pixelSize)) || math.IsInf(float64(pixelSize), 0) {
		panic("ink: invalid pixel size")
	}
	for _, p := range points {
		for _, v := range []float32{p.X, p.Y, p.Radius} {
			if math.IsNaN(float64(v)) || math.IsInf(float64(v), 0) {
				panic("ink: non-finite point")
			}
		}
		if p.Radius < 0 {
			panic("ink: negative radius")
		}
	}
	if len(points) == 1 {
		points = []Point{points[0], points[0], points[0]}
	}
	if len(points) == 2 {
		a, b := points[0], points[1]
		points = []Point{a, {(a.X + b.X) / 2, (a.Y + b.Y) / 2, (a.Radius + b.Radius) / 2}, b}
	}
	for i := 0; i+2 < len(points); i++ {
		a, b, c := points[i], points[i+1], points[i+2]
		ab, cb, ac := length(a.X-b.X, a.Y-b.Y), length(c.X-b.X, c.Y-b.Y), length(c.X-a.X, c.Y-a.Y)
		mode := 0
		if (ab < 0.05 && cb < 0.05) || ac+min(a.Radius, c.Radius) <= max(a.Radius, c.Radius) {
			mode = 2
		} else if ac == 0 || float32(math.Abs(float64((b.X-a.X)*(c.Y-a.Y)-(b.Y-a.Y)*(c.X-a.X))))/(ac*ac) < 0.00005 || ab < 0.05 || cb < 0.05 {
			mode = 1
		}
		if ab < 0.05 || cb < 0.05 {
			b = Point{(a.X + c.X) / 2, (a.Y + c.Y) / 2, (a.Radius + c.Radius) / 2}
		}
		// Convex hull expanded by max radius: conservative even with phase remapping.
		r := max(a.Radius, b.Radius, c.Radius) + pixelSize
		x0, y0 := min(a.X, b.X, c.X)-r, min(a.Y, b.Y, c.Y)-r
		x1, y1 := max(a.X, b.X, c.X)+r, max(a.Y, b.Y, c.Y)+r
		h := y1 - y0
		k := 2 / h
		norm := func(p Point) Point { return Point{(p.X-x0)*k - 1, (p.Y-y0)*k - 1, p.Radius * k} }
		s := Segment{P: [3]Point{norm(a), norm(b), norm(c)}, Bounds: [4]float32{x0 + translation[0], y0 + translation[1], x1 - x0, h}, Mode: mode}
		p, q, e := s.P[0], s.P[1], s.P[2]
		if mode != 2 {
			s.Phase = [2]float32{phaseOffset(p, q), phaseOffset(e, q)}
		}
		s.A = [2]float32{q.X - p.X, q.Y - p.Y}
		s.B = [2]float32{p.X - 2*q.X + e.X, p.Y - 2*q.Y + e.Y}
		s.KK = 1 / max(s.B[0]*s.B[0]+s.B[1]*s.B[1], 1e-6)
		s.KX = s.KK * (s.A[0]*s.B[0] + s.A[1]*s.B[1])
		out = append(out, s)
	}
	return out
}

// Radius is a CPU pressure/speed model; geometric inputs already contain radii.
func Radius(width, pressure, speed, weight float32) float32 {
	r := width / 2 * ((1 - weight) + weight*(0.35+1.15*clamp(pressure, 0, 1))*(0.7+0.6/(1+max(speed, 0)/900)))
	if r < 0.35 {
		return 0
	}
	return r
}

// SampleCurves turns pointer samples into a connected quadratic chain. Every
// interior join is a shared midpoint in x/y/radius with aligned tangents.
// Interior quadratic pieces also share equal derivatives; end pieces are lines.
// Keep the leading and trailing half-intervals so the stroke reaches both ends.
func SampleCurves(points []Point) [][3]Point {
	if len(points) == 0 {
		return nil
	}
	if len(points) == 1 {
		return [][3]Point{{points[0], points[0], points[0]}}
	}
	if len(points) == 2 {
		return [][3]Point{{points[0], midpoint(points[0], points[1]), points[1]}}
	}
	curves := make([][3]Point, 0, len(points))
	for i := range points {
		curves = append(curves, sampleCurve(points, i))
	}
	return curves
}
func midpoint(a, b Point) Point {
	return Point{(a.X + b.X) / 2, (a.Y + b.Y) / 2, (a.Radius + b.Radius) / 2}
}
func strokeSegments(s Stroke, pixelSize float32) []Segment {
	if !s.Smooth {
		return Segments(s.Points, s.Translation, pixelSize)
	}
	if len(s.Points) < 3 {
		return Segments(s.Points, s.Translation, pixelSize)
	}
	out := make([]Segment, 0, len(s.Points))
	for i := range s.Points {
		curve := sampleCurve(s.Points, i)
		out = appendSegments(out, curve[:], s.Translation, pixelSize)
	}
	return out
}

// sampleCurve builds just one piece, so live drawing does not rescan history.
func sampleCurve(points []Point, i int) [3]Point {
	start := points[0]
	if i > 0 {
		start = midpoint(points[i-1], points[i])
	}
	end := points[len(points)-1]
	if i+1 < len(points) {
		end = midpoint(points[i], points[i+1])
	}
	control := points[i]
	if i == 0 || i == len(points)-1 {
		control = midpoint(start, end)
	}
	return [3]Point{start, control, end}
}
