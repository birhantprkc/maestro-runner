package devicelab_ios

import "testing"

// TestTextEntryLanded covers the iOS analogue of the Android keyPress
// misdirection (#139): the runner types somewhere, reports success, and the
// field the flow named never receives the text.
//
// iOS gives the host no focus signal to check beforehand — the runner leaves
// SnapshotNode.Focused unpopulated — so the only available evidence is whether
// the field's value moved.
func TestTextEntryLanded(t *testing.T) {
	tests := []struct {
		name   string
		before string
		after  string
		typed  string
		want   bool
	}{
		{"value changed", "", "HelloWorld", "HelloWorld", true},
		{"appended to existing text", "abc", "abcdef", "def", true},
		// A secure field renders dots rather than the characters typed; the
		// value still moves, which is all this check needs.
		{"secure field shows a mask", "", "••••••", "hunter2", true},
		// Controlled inputs may transform what was typed (stripping spaces,
		// changing case). Still a change, so still landed.
		{"transformed input", "", "Itemone", "Item one", true},

		// The failure being caught: nothing moved.
		{"unchanged means nothing landed", "", "", "HelloWorld", false},
		// Placeholders live in PlaceholderValue, not Value, so an untouched
		// field reads as empty here rather than as its placeholder text.
		{"unchanged with prior content", "abc", "abc", "xyz", false},

		// Re-running a flow without clearState legitimately types a value the
		// field already holds; unchanged is correct there, not a failure.
		{"retyping identical content", "HelloWorld", "HelloWorld", "HelloWorld", true},
		{"retyping a substring already present", "HelloWorld", "HelloWorld", "World", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := textEntryLanded(tt.before, tt.after, tt.typed); got != tt.want {
				t.Errorf("textEntryLanded(%q, %q, %q) = %v, want %v",
					tt.before, tt.after, tt.typed, got, tt.want)
			}
		})
	}
}

// Verification is best-effort: with nothing to re-read it must stay silent
// rather than invent a failure, since a false failure is worse than the silent
// success it replaces.
func TestVerifyTextEntrySkipsWithoutIdentifier(t *testing.T) {
	d := &Driver{}
	if err := d.verifyTextEntry("", "before", "typed"); err != nil {
		t.Errorf("expected no verification without an identifier, got %v", err)
	}
}
