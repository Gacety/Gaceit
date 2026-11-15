package nativeapp

import (
	"github.com/egoist/mygo/ui"
	"slices"
	"testing"
	"time"
)

func TestAnswerCardVirtualRowsFollowCurrentQuestion(t *testing.T) {
	a := testApp(t)
	tt := ui.NewTester(a.View, 1440, 900)
	if _, ok := tt.Find("答题卡 第2题"); !ok {
		t.Fatal("first visible answer row missing")
	}
	if _, ok := tt.Find("答题卡 第1000题"); ok {
		t.Fatal("offscreen answer rows were eagerly built")
	}
	id := a.queue[999]
	a.open(id)
	tt.Frame()
	if _, ok := tt.Find("答题卡 第1000题"); !ok {
		t.Fatal("virtual card did not follow current question")
	}
	if _, ok := tt.Find("答题卡 第2题"); ok {
		t.Fatal("distant old rows retained")
	}
	if err := tt.Click("答题卡 第1001题"); err != nil {
		t.Fatal(err)
	}
	if a.M.Round.CurrentID != a.queue[1000] {
		t.Fatal("virtual cell changed stable round mapping")
	}
}
func TestRichCacheBoundsAndReconstruction(t *testing.T) {
	c := richCache[string]{maxEntries: 4, maxBytes: 40}
	for i := 0; i < 1000; i++ {
		key := time.Unix(int64(i), 0).String()
		c.set(key, "body", 10)
	}
	if len(c.items) > 4 || c.bytes > 40 {
		t.Fatal("render cache grew beyond budget")
	}
	c.set("oversize", "body", 100)
	if _, ok := c.get("oversize"); ok {
		t.Fatal("oversize value bypassed budget")
	}
	c.set("a", "reconstructed", 10)
	if value, ok := c.get("a"); !ok || value != "reconstructed" {
		t.Fatal("evicted render data cannot be reconstructed")
	}
}
func TestQueueRetainsOrderAndAppendsOnlyNewArrivals(t *testing.T) {
	a := testApp(t)
	m := a.M
	m.Round.Mode = "wrong"
	m.Round.WrongScope = "all"
	now := time.Now()
	first := m.Queue("全部学科", "全部章节", now)
	for i := 0; i < 5; i++ {
		if !slices.Equal(first, m.Queue("全部学科", "全部章节", now)) {
			t.Fatal("ordinary queue read reordered round")
		}
	}
	var newID string
	for _, q := range m.Questions {
		if !m.Records[q.ID].Wrong {
			newID = q.ID
			break
		}
	}
	m.Records[newID].Wrong = true
	next := m.Queue("全部学科", "全部章节", now)
	if !slices.Equal(next[:len(first)], first) || next[len(next)-1] != newID {
		t.Fatal("arrival renumbered existing round")
	}
	m.Round.Mode = "random"
	m.Round.RandomIDs = []string{newID, first[0], newID, "missing", first[1]}
	if got := m.Queue("全部学科", "全部章节", now); !slices.Equal(got, []string{newID, first[0], first[1]}) {
		t.Fatal("random dedup/order changed", got)
	}
}
