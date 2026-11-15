package inkcanvas

// update changes only cells belonging to changed positional IDs. Appending a
// stroke never rebuilds the index for the existing document.
func (idx *panIndex) update(before, after []panDrawable) {
	if idx.cells == nil {
		idx.build(after)
		return
	}
	remove := func(id int, b [4]float32) {
		for i, v := range idx.large {
			if v == id {
				idx.large = append(idx.large[:i], idx.large[i+1:]...)
				return
			}
		}
		x0, y0, x1, y1 := panCellRange(b)
		for x := x0; x <= x1; x++ {
			for y := y0; y <= y1; y++ {
				key := panCell{x, y}
				ids := idx.cells[key]
				for i, v := range ids {
					if v == id {
						ids = append(ids[:i], ids[i+1:]...)
						idx.entries--
						break
					}
				}
				if len(ids) == 0 {
					delete(idx.cells, key)
				} else {
					idx.cells[key] = ids
				}
			}
		}
	}
	for id, s := range before {
		if id >= len(after) || s.bounds != after[id].bounds {
			remove(id, s.bounds)
		}
	}
	if len(idx.seen) < len(after) {
		idx.seen = append(idx.seen, make([]uint32, len(after)-len(idx.seen))...)
	} else {
		idx.seen = idx.seen[:len(after)]
	}
	for id, s := range after {
		if id < len(before) && s.bounds == before[id].bounds {
			continue
		}
		x0, y0, x1, y1 := panCellRange(s.bounds)
		w, h := x1-x0+1, y1-y0+1
		if w <= 0 || h <= 0 || w > 64 || h > 64 || w*h > 64 || idx.entries+int(w*h) > panIndexEntryBudget {
			idx.large = append(idx.large, id)
			continue
		}
		for x := x0; x <= x1; x++ {
			for y := y0; y <= y1; y++ {
				key := panCell{x, y}
				idx.cells[key] = append(idx.cells[key], id)
				idx.entries++
			}
		}
	}
}
