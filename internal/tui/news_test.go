package tui

import (
	"strings"
	"testing"

	"github.com/moneytosms/verd/internal/relnotes"
)

func TestWhatsNewOpensAndCloses(t *testing.T) {
	mm, _ := New(nil, "", Deps{}).Update(whatsNewMsg{[]relnotes.Release{{Tag: "v0.8.4", Body: "## New\n- thing"}}})
	m := mm.(Model)
	if m.news == nil || !strings.Contains(strings.Join(m.modalBox(), "\n"), "What's new") {
		t.Fatal("modal not shown")
	}
	m = sendKeys(m, "enter")
	if m.news != nil {
		t.Fatal("enter did not close")
	}
}
