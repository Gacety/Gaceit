package ink

// PencilStream retains the exact RNG and arc-length spacing at immutable
// curve boundaries. The changing final half-curve is generated from a copy.
// Raw samples, pressure, curve math and stamp order are unchanged.
type PencilStream struct {
	sampler       stampSampler
	stable        []Stamp
	tail          []Stamp
	pieces, count int
	last          Point
	smooth        bool
}

func (g *PencilStream) Update(s Stroke) (stable, tail []Stamp) {
	s.Translation = [2]float32{}
	n := len(s.Points)
	if n == 0 {
		return nil, nil
	}
	if n == g.count && g.smooth == s.Smooth && s.Points[n-1] == g.last {
		return g.stable, g.tail
	}
	if g.sampler.rects == nil || n < g.count || s.Smooth != g.smooth || n == g.count && s.Points[n-1] != g.last {
		rects, err := atlasRects()
		if err != nil {
			panic(err)
		}
		*g = PencilStream{sampler: stampSampler{seed: 1, rects: rects}, smooth: s.Smooth}
	}
	if n < 3 {
		temp := g.sampler
		g.tail = temp.append(g.tail[:0], strokeSegments(s, 1))
	} else {
		target := n - 2
		if s.Smooth {
			target = n - 1
		}
		for g.pieces < target {
			var segs []Segment
			if s.Smooth {
				curve := sampleCurve(s.Points, g.pieces)
				segs = Segments(curve[:], [2]float32{}, 1)
			} else {
				segs = Segments(s.Points[g.pieces:g.pieces+3], [2]float32{}, 1)
			}
			g.stable = g.sampler.append(g.stable, segs)
			g.pieces++
		}
		g.tail = g.tail[:0]
		if s.Smooth {
			temp := g.sampler
			curve := sampleCurve(s.Points, n-1)
			g.tail = temp.append(g.tail, Segments(curve[:], [2]float32{}, 1))
		}
	}
	g.count = n
	g.last = s.Points[n-1]
	return g.stable, g.tail
}

// Finish also consumes samples that arrived after the last paint callback.
func (g *PencilStream) Finish(s Stroke) []Stamp {
	stable, tail := g.Update(s)
	out := make([]Stamp, 0, len(stable)+len(tail))
	out = append(out, stable...)
	return append(out, tail...)
}
