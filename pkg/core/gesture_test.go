package core

import "testing"

// TestSwipeCoordsInBounds locks in the element-anchored swipe coordinates
// (start inside the element, end past the opposite edge, clamped to the
// screen). These feed the drivers' absolute-coordinate swipe paths so a
// from:/selector swipe honours duration: (#114).
func TestSwipeCoordsInBounds(t *testing.T) {
	b := Bounds{X: 0, Y: 100, Width: 500, Height: 800}
	screenW, screenH := 1080, 2400
	cases := []struct {
		dir            string
		sx, sy, ex, ey int
	}{
		{"left", 450, 500, 0, 500},   // end clamps to 0 (element flush against left edge)
		{"right", 50, 500, 550, 500}, // end 10% past right edge
		{"up", 250, 820, 250, 20},    // end 10% above top (100 - 80)
		{"down", 250, 180, 250, 980}, // end 10% below bottom
	}
	for _, c := range cases {
		t.Run(c.dir, func(t *testing.T) {
			sx, sy, ex, ey, err := SwipeCoordsInBounds(c.dir, b, screenW, screenH)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if sx != c.sx || sy != c.sy || ex != c.ex || ey != c.ey {
				t.Errorf("SwipeCoordsInBounds(%q) = (%d,%d)->(%d,%d), want (%d,%d)->(%d,%d)",
					c.dir, sx, sy, ex, ey, c.sx, c.sy, c.ex, c.ey)
			}
		})
	}
}

// TestSwipeCoordsInBounds_ClampsToScreen verifies overshoot past the far
// screen edge lands on the last on-screen pixel for elements flush against
// the right/bottom edges.
func TestSwipeCoordsInBounds_ClampsToScreen(t *testing.T) {
	screenW, screenH := 1000, 2000
	// Element flush against the right and bottom screen edges.
	b := Bounds{X: 500, Y: 1200, Width: 500, Height: 800}

	_, _, ex, _, err := SwipeCoordsInBounds("right", b, screenW, screenH)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ex != screenW-1 {
		t.Errorf("right swipe end X = %d, want clamp to %d", ex, screenW-1)
	}

	_, _, _, ey, err := SwipeCoordsInBounds("down", b, screenW, screenH)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ey != screenH-1 {
		t.Errorf("down swipe end Y = %d, want clamp to %d", ey, screenH-1)
	}
}

// TestSwipeCoordsInBounds_UnknownScreen verifies that unknown screen dims
// (0,0) skip the far-edge clamp but still clamp negatives to 0.
func TestSwipeCoordsInBounds_UnknownScreen(t *testing.T) {
	b := Bounds{X: 0, Y: 100, Width: 500, Height: 800}
	sx, sy, ex, ey, err := SwipeCoordsInBounds("right", b, 0, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sx != 50 || sy != 500 || ex != 550 || ey != 500 {
		t.Errorf("got (%d,%d)->(%d,%d), want (50,500)->(550,500)", sx, sy, ex, ey)
	}

	_, _, ex, _, err = SwipeCoordsInBounds("left", b, 0, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ex != 0 {
		t.Errorf("left swipe end X = %d, want negative overshoot clamped to 0", ex)
	}
}

func TestNormalizeSwipeDirection(t *testing.T) {
	good := map[string]string{"": "up", "up": "up", "DOWN": "down", "Left": "left", "right": "right"}
	for in, want := range good {
		got, err := NormalizeSwipeDirection(in)
		if err != nil || got != want {
			t.Errorf("NormalizeSwipeDirection(%q) = (%q, %v), want (%q, nil)", in, got, err, want)
		}
	}
	for _, in := range []string{"diagonal", "upp", "north"} {
		if _, err := NormalizeSwipeDirection(in); err == nil {
			t.Errorf("NormalizeSwipeDirection(%q) expected error, got nil", in)
		}
	}
}

func TestSwipeCoordsInBounds_InvalidDirection(t *testing.T) {
	for _, dir := range []string{"", "diagonal", "UP"} {
		if _, _, _, _, err := SwipeCoordsInBounds(dir, Bounds{Width: 10, Height: 10}, 100, 100); err == nil {
			t.Errorf("SwipeCoordsInBounds(%q) expected error, got nil", dir)
		}
	}
}

func TestDirectionSwipeScreenCoords(t *testing.T) {
	// distance 0.5 on a 1000x1000 screen → centered swipe of 500px total.
	sx, sy, ex, ey, err := DirectionSwipeScreenCoords("up", 1000, 1000, 0.5)
	if err != nil {
		t.Fatal(err)
	}
	if sx != 500 || sy != 750 || ex != 500 || ey != 250 {
		t.Errorf("up 0.5 = (%d,%d)->(%d,%d), want (500,750)->(500,250)", sx, sy, ex, ey)
	}
	// distance clamps: 0 defaults to 0.5, >1 clamps to 1.
	if _, _, _, ey0, _ := DirectionSwipeScreenCoords("up", 1000, 1000, 0); ey0 != 250 {
		t.Errorf("distance 0 should default to 0.5")
	}
	if _, _, _, ey1, _ := DirectionSwipeScreenCoords("up", 1000, 1000, 2); ey1 != 0 {
		t.Errorf("distance 2 should clamp to full screen, ey=%d", ey1)
	}
	if _, _, _, _, err := DirectionSwipeScreenCoords("sideways", 100, 100, 0.5); err == nil {
		t.Error("invalid direction should error")
	}
}

func TestSwipeCoordsFromBounds(t *testing.T) {
	// A small anchor near the middle of a 1080x2340 screen — the shape that
	// made `swipe: from:` useless for scrolling in #141.
	anchor := Bounds{X: 113, Y: 1401, Width: 856, Height: 77}

	t.Run("travel comes from distance, not the anchor's size", func(t *testing.T) {
		_, startY, _, endY, err := SwipeCoordsFromBounds("up", anchor, 1080, 2340, 0.5)
		if err != nil {
			t.Fatalf("SwipeCoordsFromBounds() error = %v", err)
		}
		travel := startY - endY
		// Half the screen, not the 77px anchor.
		if travel != 1170 {
			t.Errorf("travel = %d, want 1170 (half of 2340)", travel)
		}
	})

	t.Run("starts at the anchor centre so the anchor captures the touch", func(t *testing.T) {
		startX, startY, _, _, err := SwipeCoordsFromBounds("down", anchor, 1080, 2340, 0.2)
		if err != nil {
			t.Fatalf("SwipeCoordsFromBounds() error = %v", err)
		}
		wantX, wantY := anchor.Center()
		if startX != wantX || startY != wantY {
			t.Errorf("start = (%d,%d), want the anchor centre (%d,%d)", startX, startY, wantX, wantY)
		}
	})

	t.Run("clamps overshoot to the screen", func(t *testing.T) {
		_, _, _, endY, err := SwipeCoordsFromBounds("down", anchor, 1080, 2340, 1)
		if err != nil {
			t.Fatalf("SwipeCoordsFromBounds() error = %v", err)
		}
		if endY != 2339 {
			t.Errorf("endY = %d, want the last on-screen pixel 2339", endY)
		}
	})

	t.Run("non-positive distance falls back to half a screen", func(t *testing.T) {
		_, startY, _, endY, err := SwipeCoordsFromBounds("up", anchor, 1080, 2340, 0)
		if err != nil {
			t.Fatalf("SwipeCoordsFromBounds() error = %v", err)
		}
		if startY-endY != 1170 {
			t.Errorf("travel = %d, want the 0.5 default (1170)", startY-endY)
		}
	})

	t.Run("rejects an unknown direction", func(t *testing.T) {
		if _, _, _, _, err := SwipeCoordsFromBounds("sideways", anchor, 1080, 2340, 0.5); err == nil {
			t.Error("expected an error for an unknown direction")
		}
	})
}
