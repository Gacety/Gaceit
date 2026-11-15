package ink

import (
	"embed"
	"encoding/json"
	"fmt"
	"math"
	"sync"
)

//go:embed assets/* *.kage
var resources embed.FS

var cachedAtlasRects = sync.OnceValues(loadAtlasRects)

func atlasRects() ([][4]int, error) { return cachedAtlasRects() }
func loadAtlasRects() ([][4]int, error) {
	data, err := resources.ReadFile("assets/stamp-rects.json")
	if err != nil {
		return nil, err
	}
	var rects [][4]int
	if err = json.Unmarshal(data, &rects); err != nil {
		return nil, err
	}
	if len(rects) != 576 {
		return nil, fmt.Errorf("ink: atlas has %d stamps, want 576", len(rects))
	}
	return rects, nil
}

// MakeStamps samples the actual quadratic paths by approximate arc length,
// carrying spacing across segment boundaries. The seed makes redraws stable.
func MakeStamps(points []Point, seed uint32) []Stamp {
	return MakeStrokeStamps(Stroke{Points: points}, seed)
}

// MakeStrokeStamps follows the same curve interpretation as Renderer.Draw.
// Translation stays on the Stroke and is applied once by the renderer.
func MakeStrokeStamps(stroke Stroke, seed uint32) []Stamp {
	if len(stroke.Points) == 0 {
		return nil
	}
	stroke.Translation = [2]float32{}
	rects, err := atlasRects()
	if err != nil {
		panic(err)
	}
	g := stampSampler{seed: seed, rects: rects}
	return g.append(nil, strokeSegments(stroke, 1))
}

type stampSampler struct {
	seed        uint32
	rects       [][4]int
	previous    Point
	remaining   float32
	hasPrevious bool
}

func (g *stampSampler) append(out []Stamp, segments []Segment) []Stamp {
	random := func() uint32 { g.seed = 1664525*g.seed + 1013904223; return g.seed }
	choose := func(p Point, angle float32) Stamp {
		target := max(1, 2*p.Radius) * 8
		best := float32(math.Inf(1))
		var candidates [576]int
		count := 0
		for i, r := range g.rects {
			d := float32(math.Abs(math.Log(float64(float32(r[3]) / target))))
			if d < best-0.15 {
				best = d
				count = 0
			}
			if d <= best+0.15 {
				candidates[count] = i
				count++
			}
		}
		idx := candidates[int(random()%uint32(count))]
		r := g.rects[idx]
		h := max(0.01, 2*p.Radius)
		w := h * float32(r[2]) / float32(r[3])
		if random()&1 != 0 {
			w = -w
		}
		return Stamp{{p.X, p.Y, 0.32, angle}, {w, h, float32(idx), 0}}
	}
	for _, s := range segments {
		// Evaluate original geometry from normalized coordinates.
		eval := func(t float32) Point {
			h := s.Bounds[3] / 2
			return Point{(Quad(s.P[0].X, s.P[1].X, s.P[2].X, t)+1)*h + s.Bounds[0], (Quad(s.P[0].Y, s.P[1].Y, s.P[2].Y, t)+1)*h + s.Bounds[1], Quad(s.P[0].Radius, s.P[1].Radius, s.P[2].Radius, t) * h}
		}
		a, b, c := eval(0), eval(0.5), eval(1)
		steps := max(1, int(math.Ceil(float64((length(b.X-a.X, b.Y-a.Y)+length(c.X-b.X, c.Y-b.Y))*4))))
		if g.hasPrevious {
			g.previous = eval(0)
		}
		for j := 0; j <= steps; j++ {
			p := eval(float32(j) / float32(steps))
			if !g.hasPrevious {
				out = append(out, choose(p, 0))
				g.previous = p
				g.hasPrevious = true
				g.remaining = max(0.5, p.Radius*0.45)
				continue
			}
			dx, dy := p.X-g.previous.X, p.Y-g.previous.Y
			d := length(dx, dy)
			for d >= g.remaining && d > 1e-7 {
				t := g.remaining / d
				q := Point{g.previous.X + dx*t, g.previous.Y + dy*t, g.previous.Radius + (p.Radius-g.previous.Radius)*t}
				out = append(out, choose(q, float32(math.Atan2(float64(dy), float64(dx)))))
				g.previous = q
				dx, dy = p.X-q.X, p.Y-q.Y
				d = length(dx, dy)
				g.remaining = max(0.5, q.Radius*0.45)
			}
			g.remaining -= d
			g.previous = p
		}
	}
	return out
}

var pencilBoundsExpansion = sync.OnceValue(func() float32 {
	rects, err := atlasRects()
	if err != nil {
		panic(err)
	}
	factor := float32(1)
	for _, r := range rects {
		factor = max(factor, 1+float32(r[2])/float32(r[3]))
	}
	return factor
})
