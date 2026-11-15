package nativeapp

import (
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

func TestDeferredSaveCoalescesTyping(t *testing.T) {
	a := testApp(t)
	id := a.M.Current().ID
	if err := a.M.Save(); err != nil {
		t.Fatal(err)
	}

	a.M.Records[id].Notes = "延迟保存内容"
	a.saveSoon()
	if !a.savePending {
		t.Fatal("edit did not arm the deferred save")
	}
	saved, err := NewModel(assetsForTest(), a.M.Disk)
	if err != nil {
		t.Fatal(err)
	}
	if saved.Records[id].Notes != "" {
		t.Fatal("deferred save wrote synchronously")
	}

	a.flushSave()
	if a.savePending {
		t.Fatal("flush left the save pending")
	}
	saved, err = NewModel(assetsForTest(), a.M.Disk)
	if err != nil {
		t.Fatal(err)
	}
	if saved.Records[id].Notes != "延迟保存内容" {
		t.Fatal("flush did not persist the edit")
	}
}

func TestImmediateSaveClearsPendingWrite(t *testing.T) {
	a := testApp(t)
	id := a.M.Current().ID
	a.M.Records[id].Notes = "立即保存"
	a.saveSoon()
	a.save()
	if a.savePending {
		t.Fatal("immediate save left a second write armed")
	}
	saved, err := NewModel(assetsForTest(), a.M.Disk)
	if err != nil {
		t.Fatal(err)
	}
	if saved.Records[id].Notes != "立即保存" {
		t.Fatal("immediate save did not persist")
	}
}

func TestDeferredSaveRunsAfterPause(t *testing.T) {
	a := testApp(t)
	id := a.M.Current().ID
	a.save()
	a.M.Records[id].Notes = "暂停后落盘"
	a.saveSoon()
	tt := ui.NewTester(a.View, 1440, 960)
	// The scheduler uses wall time, as the native host does; advance real time
	// and request the frame that services the pending write.
	time.Sleep(650 * time.Millisecond)
	tt.Frame()
	a.collectSave(true)
	saved, err := NewModel(assetsForTest(), a.M.Disk)
	if err != nil {
		t.Fatal(err)
	}
	if saved.Records[id].Notes != "暂停后落盘" {
		t.Fatal("deferred save did not run after the pause")
	}
}
