package tui

import (
	"strings"
	"testing"

	"github.com/moneytosms/verd/internal/relnotes"
)

func TestUpdateAskYesUpdatesNoLeavesHint(t *testing.T) {
	updated := 0
	deps := Deps{Update: func() (string, error) {
		updated++
		return "updated verd 0.8.3 -> 0.8.4", nil
	}}
	mm, _ := New(nil, "", deps).Update(updateAvailMsg{"v0.8.4"})
	m := mm.(Model)
	if !strings.Contains(strings.Join(m.modalBox(), "\n"), "Update verd") {
		t.Fatal("no update modal")
	}

	no, _ := send(m, "n")
	if no.updHint != "v0.8.4" || no.updAsk != "" || no.modalBox() != nil || !strings.Contains(no.viewToast(), "run verd update") {
		t.Fatalf("n: hint %q ask %q", no.updHint, no.updAsk)
	}

	yes, cmd := send(m, "y")
	if cmd == nil {
		t.Fatal("y: no update command")
	}
	done, _ := yes.Update(cmd())
	if updated != 1 || done.(Model).toast == nil || !strings.Contains(done.(Model).toast.text, "updated verd") {
		t.Fatalf("y: calls %d", updated)
	}
}

func TestUpdateAskWaitsForOtherModals(t *testing.T) {
	deps := Deps{Update: func() (string, error) { return "", nil }}
	mm, _ := New(nil, "", deps).Update(whatsNewMsg{[]relnotes.Release{{Tag: "v0.8.3", Body: "- x"}}})
	mm, _ = mm.(Model).Update(updateAvailMsg{"v0.8.4"})
	m := mm.(Model)
	if got := strings.Join(m.modalBox(), "\n"); strings.Contains(got, "Update verd") || !strings.Contains(got, "What's new") {
		t.Fatal("update asked over What's new")
	}
	m = sendKeys(m, "esc")
	if !strings.Contains(strings.Join(m.modalBox(), "\n"), "Update verd") {
		t.Fatal("update not asked after What's new closed")
	}
}
