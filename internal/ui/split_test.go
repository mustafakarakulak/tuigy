package ui

import "testing"

// The body is shared so that both boxes can always be drawn, at every size the
// interface is asked to work at.
func TestSplitBodyAlwaysLeavesRoomForBoth(t *testing.T) {
	for bodyH := 1; bodyH <= 80; bodyH++ {
		panesH, termH := splitBody(bodyH, true)

		if panesH+termH != bodyH {
			t.Fatalf("bodyH=%d split into %d+%d, which is not %d", bodyH, panesH, termH, bodyH)
		}
		if termH < minBoxHeight && bodyH >= minBoxHeight {
			t.Errorf("bodyH=%d gave the shell %d rows, too few to draw a box", bodyH, termH)
		}
		if bodyH >= minBoxHeight*2 {
			if panesH < minBoxHeight {
				t.Errorf("bodyH=%d left the panes %d rows, too few to draw a box", bodyH, panesH)
			}
			if termH > bodyH/2 {
				t.Errorf("bodyH=%d gave the shell %d rows, more than half", bodyH, termH)
			}
		}
	}

	if panesH, termH := splitBody(30, false); panesH != 30 || termH != 0 {
		t.Errorf("with no shell the split is %d+%d, want the whole body to the panes", panesH, termH)
	}
}
