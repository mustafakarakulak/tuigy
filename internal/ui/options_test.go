package ui

import (
	"strings"
	"testing"

	"github.com/mustafakarakulak/tuigy/internal/keys"
)

// A key map from a config file has to reach the running model, and the footer
// has to advertise the keys that actually work.
func TestWithKeysRebindsTheRunningModel(t *testing.T) {
	km := keys.Default()
	if err := km.Apply(map[string][]string{"commit": {"ctrl+k"}}); err != nil {
		t.Fatalf("Apply: %v", err)
	}

	m, _ := newTestModel(t, 120, 32, WithKeys(km))

	// The default key no longer does anything.
	after, _ := m.press(t, "c")
	if after.modal == modalCommit {
		t.Error("the rebound action still responds to its old key")
	}

	after, _ = m.press(t, "ctrl+k")
	if after.modal != modalCommit {
		t.Fatal("the new key does not open the commit view")
	}
	if !strings.Contains(m.footerView(), "ctrl+k commit") {
		t.Errorf("the footer does not advertise the configured key:\n%s", m.footerView())
	}
}

func TestWithAIDisablesTheGenerator(t *testing.T) {
	withFakeAgent(t, "printf 'feat: x\\n'")

	m, _ := newTestModel(t, 120, 32, WithAI(nil))
	if m.ai != nil {
		t.Fatal("WithAI(nil) should disable the generator")
	}

	m, _ = m.press(t, "c")
	if strings.Contains(m.View(), "ctrl+g") {
		t.Error("a disabled generator should not be advertised")
	}
}
