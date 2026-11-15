package nativeapp

import (
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

func TestLongPressExcludesAndRestoresOption(t *testing.T) {
	a := testApp(t)
	tt := ui.NewTester(a.View, 1440, 900)
	rect, ok := tt.Find("第1空 选项A")
	if !ok {
		t.Fatal("option A not found")
	}
	x, y := rect.X+rect.W/2, rect.Y+rect.H/2
	q := a.M.Current()
	if a.optionExcluded(q.ID, 0, "A") {
		t.Fatal("option unexpectedly excluded before long press")
	}

	tt.Press(x, y)
	time.Sleep(longPressDelay + 150*time.Millisecond)
	tt.Frame()
	if !a.optionExcluded(q.ID, 0, "A") {
		t.Fatal("long press did not exclude option A")
	}
	tt.Release(x, y)
	if a.choices[0] != "" {
		t.Fatalf("long press selected the excluded option: %q", a.choices[0])
	}

	tt.Press(x, y)
	time.Sleep(longPressDelay + 150*time.Millisecond)
	tt.Frame()
	tt.Release(x, y)
	time.Sleep(80 * time.Millisecond)
	tt.Frame()
	if a.optionExcluded(q.ID, 0, "A") {
		t.Fatal("second long press did not restore option A")
	}

	a.flushSave()
	m, err := NewModel(assetsForTest(), a.M.Disk)
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Records[q.ID].Excluded) != 0 {
		t.Fatal("restored exclusion was persisted")
	}
}

func TestLongPressExclusionClearsOnReopen(t *testing.T) {
	a := testApp(t)
	tt := ui.NewTester(a.View, 1440, 900)
	rect, _ := tt.Find("第1空 选项B")
	x, y := rect.X+rect.W/2, rect.Y+rect.H/2
	q := a.M.Current()
	tt.Press(x, y)
	time.Sleep(longPressDelay + 150*time.Millisecond)
	tt.Frame()
	tt.Release(x, y)
	a.flushSave()
	m, err := NewModel(assetsForTest(), a.M.Disk)
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Records[q.ID].Excluded) != 1 {
		t.Fatal("setup: missing saved exclusion")
	}
	// Switching tools within this visit must not reset the reader.
	a.SetTool("scratch")
	a.SetTool("notes")
	if !a.optionExcluded(q.ID, 0, "B") {
		t.Fatal("tool switch cleared current exclusion")
	}
	a.open("relax-cn-01-001")
	a.open(q.ID)
	if a.optionExcluded(q.ID, 0, "B") {
		t.Fatal("returning to question retained exclusion")
	}
	// Legacy stored exclusions are cleared on application startup as well.
	m.Records[q.ID].Excluded = []string{optionExKey(0, "B")}
	if err := m.Save(); err != nil {
		t.Fatal(err)
	}
	b, err := New(assetsForTest(), a.M.Disk)
	if err != nil {
		t.Fatal(err)
	}
	if b.optionExcluded(q.ID, 0, "B") {
		t.Fatal("startup restored old exclusion")
	}
	b.toggleOptionExclusion(q.ID, 0, "B")
	b.open(q.ID)
	if b.optionExcluded(q.ID, 0, "B") {
		t.Fatal("explicitly reopening same question retained exclusion")
	}
	b.flushSave()
	saved, err := NewModel(assetsForTest(), a.M.Disk)
	if err != nil {
		t.Fatal(err)
	}
	if len(saved.Records[q.ID].Excluded) != 0 {
		t.Fatal("reset did not clear saved field")
	}

}

func TestOnlyLongPressRestoresExcludedOption(t *testing.T) {
	a := testApp(t)
	tt := ui.NewTester(a.View, 1440, 900)
	rect, _ := tt.Find("第1空 选项C")
	x, y := rect.X+rect.W/2, rect.Y+rect.H/2
	q := a.M.Current()
	tt.Press(x, y)
	time.Sleep(longPressDelay + 150*time.Millisecond)
	tt.Frame()
	tt.Release(x, y)
	time.Sleep(80 * time.Millisecond)
	tt.Frame()
	if !a.optionExcluded(q.ID, 0, "C") {
		t.Fatal("setup: C was not excluded")
	}
	if err := tt.Click("第1空 选项C"); err != nil {
		t.Fatal(err)
	}
	if a.choices[0] != "" {
		t.Fatalf("restoring exclusion selected it: %q", a.choices[0])
	}
	if !a.optionExcluded(q.ID, 0, "C") {
		t.Fatal("short click restored excluded option")
	}
	tt.Press(x, y)
	a.pressAt = time.Now().Add(-longPressDelay - time.Millisecond)
	tt.Frame()
	tt.Release(x, y)
	if a.optionExcluded(q.ID, 0, "C") || a.choices[0] != "" {
		t.Fatal("long press did not restore without selecting")
	}
	if err := tt.Click("第1空 选项C"); err != nil {
		t.Fatal(err)
	}
	if a.choices[0] != "C" {
		t.Fatal("second click did not select restored option")
	}
}

func TestExcludingSelectedOptionDoesNotFlashSelection(t *testing.T) {
	a := testApp(t)
	tt := ui.NewTester(a.View, 1440, 900)
	if err := tt.Click("第1空 选项B"); err != nil {
		t.Fatal(err)
	}
	if a.choices[0] != "B" {
		t.Fatal("setup: B not selected")
	}
	rect, _ := tt.Find("第1空 选项B")
	x, y := rect.X+rect.W/2, rect.Y+rect.H/2
	q := a.M.Current()
	tt.Press(x, y)
	time.Sleep(longPressDelay + 150*time.Millisecond)
	tt.Frame()
	tt.Release(x, y)
	if a.choices[0] != "" {
		t.Fatalf("excluding the selected option left it selected: %q", a.choices[0])
	}
	if !a.optionExcluded(q.ID, 0, "B") {
		t.Fatal("selected option was not excluded")
	}
}

func TestExcludingAnotherOptionKeepsSelection(t *testing.T) {
	a := testApp(t)
	tt := ui.NewTester(a.View, 1440, 900)
	if err := tt.Click("第1空 选项B"); err != nil {
		t.Fatal(err)
	}
	rect, _ := tt.Find("第1空 选项C")
	x, y := rect.X+rect.W/2, rect.Y+rect.H/2
	q := a.M.Current()
	tt.Press(x, y)
	time.Sleep(longPressDelay + 150*time.Millisecond)
	tt.Frame()
	tt.Release(x, y)
	if a.choices[0] != "B" {
		t.Fatalf("excluding C disturbed the selected B: %q", a.choices[0])
	}
	if !a.optionExcluded(q.ID, 0, "C") {
		t.Fatal("C was not excluded")
	}
}

func TestLongPressMovementAfterExclusionCannotSelect(t *testing.T) {
	for _, canvas := range []bool{false, true} {
		name := "reader"
		if canvas {
			name = "canvas"
		}
		t.Run(name, func(t *testing.T) {
			a := testApp(t)
			if canvas {
				a.SetTool("scratch")
			}
			tt := ui.NewTester(a.View, 1440, 900)
			rect, ok := tt.Find("第1空 选项B")
			if !ok {
				t.Fatal("option missing")
			}
			x, y := rect.X+rect.W/2, rect.Y+rect.H/2
			tt.Press(x, y)
			a.pressAt = time.Now().Add(-longPressDelay - time.Millisecond)
			tt.Frame()
			id := a.M.Current().ID
			if !a.optionExcluded(id, 0, "B") {
				t.Fatal("setup: exclusion did not fire")
			}
			// Tablet/mouse movement after the hold has already completed must not
			// turn the same gesture back into an ordinary option click.
			tt.Move(x+12, y)
			tt.Frame()
			tt.Release(x+12, y)
			if a.choices[0] != "" || !a.optionExcluded(id, 0, "B") {
				t.Fatal("long press selected or restored its option")
			}
			// Short clicks do nothing; only another hold can restore the option.
			if err := tt.Click("第1空 选项B"); err != nil {
				t.Fatal(err)
			}
			if a.choices[0] != "" || !a.optionExcluded(id, 0, "B") {
				t.Fatal("short click changed excluded option")
			}
			tt.Press(x, y)
			a.pressAt = time.Now().Add(-longPressDelay - time.Millisecond)
			tt.Frame()
			tt.Release(x, y)
			if a.choices[0] != "" || a.optionExcluded(id, 0, "B") {
				t.Fatal("long press did not restore option")
			}
			if err := tt.Click("第1空 选项B"); err != nil {
				t.Fatal(err)
			}
			if a.choices[0] != "B" {
				t.Fatal("restored option not selectable")
			}
		})
	}
}

func TestOptionHoldReleaseBeforeTimerBuild(t *testing.T) {
	for _, cancel := range []bool{false, true} {
		name := "release"
		if cancel {
			name = "canceled"
		}
		t.Run(name, func(t *testing.T) {
			a := testApp(t)
			tt := ui.NewTester(a.View, 1440, 900)
			r, _ := tt.Find("第1空 选项C")
			x, y := r.X+r.W/2, r.Y+r.H/2
			tt.Press(x, y)
			a.pressAt = time.Now().Add(-longPressDelay - time.Millisecond)
			if cancel {
				tt.CancelPress()
			} else {
				tt.Release(x, y)
			}
			if a.choices[0] != "" {
				t.Fatal("hold selected an option")
			}
			if got := a.optionExcluded(a.M.Current().ID, 0, "C"); got == cancel {
				t.Fatal("release/cancellation exclusion mismatch")
			}
		})
	}
}

func TestExcludedOptionShortcutKeepsExistingChoice(t *testing.T) {
	a := testApp(t)
	tt := ui.NewTester(a.View, 1440, 900)
	if err := tt.Click("第1空 选项A"); err != nil {
		t.Fatal(err)
	}
	id := a.M.Current().ID
	a.toggleOptionExclusion(id, 0, "B")
	tt.Frame()
	tt.Key(0, ui.Key2)
	if a.choices[0] != "A" || !a.optionExcluded(id, 0, "B") {
		t.Fatal("shortcut selected excluded option")
	}
	if err := tt.Click("第1空 选项B"); err != nil {
		t.Fatal(err)
	}
	if a.choices[0] != "A" || !a.optionExcluded(id, 0, "B") {
		t.Fatal("short click changed exclusion or existing choice")
	}
}
