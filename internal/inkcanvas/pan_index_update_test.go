package inkcanvas

import (
	"reflect"
	"testing"
)

func TestIncrementalPanIndexMatchesRebuild(t *testing.T) {
	var incremental panIndex
	var before []panDrawable
	for step := 0; step < 80; step++ {
		after := append([]panDrawable(nil), before...)
		if step%3 == 0 && len(after) > 0 {
			after = after[1:]
		} else {
			v := float32(step * 40)
			after = append(after, panDrawable{bounds: [4]float32{v, -v, v + 140, -v + 300}})
		}
		if len(after) > 0 && step%5 == 0 {
			after[0].bounds = [4]float32{-100000, -100000, 100000, 100000}
		}
		incremental.update(before, after)
		var full panIndex
		full.build(after)
		for x := -200; x < 3000; x += 113 {
			b := [4]float32{float32(x), -3000, float32(x + 250), 3000}
			got := append([]int(nil), incremental.query(b, after)...)
			want := append([]int(nil), full.query(b, after)...)
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("step %d got %v want %v", step, got, want)
			}
		}
		if incremental.entries > panIndexEntryBudget {
			t.Fatal("index budget exceeded")
		}
		before = after
	}
}
