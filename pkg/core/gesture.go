package core

import (
	"fmt"
	"strings"
)

// NormalizeSwipeDirection lowercases a YAML `direction:` value, defaults
// empty to "up" (Maestro's default), and rejects anything that isn't a
// cardinal direction so flow typos surface as errors instead of a silent
// full-screen guess.
func NormalizeSwipeDirection(s string) (string, error) {
	d := strings.ToLower(s)
	if d == "" {
		d = "up"
	}
	switch d {
	case "up", "down", "left", "right":
		return d, nil
	}
	return "", fmt.Errorf("invalid swipe direction: %q", s)
}

// SwipeCoordsInBounds returns absolute start/end coordinates for a
// direction-based swipe anchored on an element. The swipe starts inside
// the element (so the touch is captured by that element) and ends past
// the opposite edge (so drag-based targets like native sliders reach
// their extreme value). This matches classic Maestro's semantics on the
// two common use cases:
//   - scroll containers: touch starts inside and moves outward, scrolling
//     the container by the drag distance
//   - drag targets (sliders, drag handles): the release position past
//     the edge pins the drag target to its extreme in that direction
//
// Coordinates are clamped to the screen: negatives to 0, and — when
// screenW/screenH are known (> 0) — overshoot past the far edge to the
// last on-screen pixel, so elements flush against any screen edge still
// produce injectable coordinates.
func SwipeCoordsInBounds(direction string, b Bounds, screenW, screenH int) (startX, startY, endX, endY int, err error) {
	clamp := func(v, max int) int {
		if v < 0 {
			return 0
		}
		if max > 0 && v > max-1 {
			return max - 1
		}
		return v
	}
	pctX := func(p int) int { return clamp(b.X+b.Width*p/100, screenW) }
	pctY := func(p int) int { return clamp(b.Y+b.Height*p/100, screenH) }
	switch direction {
	case "up":
		return pctX(50), pctY(90), pctX(50), pctY(-10), nil
	case "down":
		return pctX(50), pctY(10), pctX(50), pctY(110), nil
	case "left":
		return pctX(90), pctY(50), pctX(-10), pctY(50), nil
	case "right":
		return pctX(10), pctY(50), pctX(110), pctY(50), nil
	default:
		return 0, 0, 0, 0, fmt.Errorf("invalid swipe direction: %q", direction)
	}
}

// DirectionSwipeScreenCoords returns start/end coordinates for a screen swipe
// covering `distance` fraction of the screen dimension, centered on screen.
// distance is clamped to (0,1]; a non-positive value defaults to 0.5. Used when
// a `swipe:` specifies an explicit `distance:` so callers control travel.
func DirectionSwipeScreenCoords(direction string, w, h int, distance float64) (startX, startY, endX, endY int, err error) {
	if distance <= 0 {
		distance = 0.5
	}
	if distance > 1 {
		distance = 1
	}
	cx, cy := w/2, h/2
	dx := int(float64(w) * distance / 2)
	dy := int(float64(h) * distance / 2)
	switch direction {
	case "up":
		return cx, cy + dy, cx, cy - dy, nil
	case "down":
		return cx, cy - dy, cx, cy + dy, nil
	case "left":
		return cx + dx, cy, cx - dx, cy, nil
	case "right":
		return cx - dx, cy, cx + dx, cy, nil
	default:
		return 0, 0, 0, 0, fmt.Errorf("invalid swipe direction: %q", direction)
	}
}

// SwipeCoordsFromBounds returns coordinates for a direction swipe anchored on
// an element that travels an explicit fraction of the screen rather than the
// element's own size.
//
// SwipeCoordsInBounds ties travel to the element: a "down" swipe runs from 10%
// to 110% of the element's height, so the gesture covers roughly one element.
// That is right for drag targets, but it makes `swipe: from:` useless for
// scrolling when the anchor is small — a 77px text input yields a ~76px drag
// that scrolls nothing. `distance:` existed to control travel but was honoured
// only for screen swipes, leaving element swipes with no control at all (#141).
//
// The swipe starts at the element's centre so the touch is captured by it, and
// travels distance × the screen dimension, clamped to the screen. distance is
// clamped to (0,1]; a non-positive value defaults to 0.5, matching
// DirectionSwipeScreenCoords.
func SwipeCoordsFromBounds(direction string, b Bounds, screenW, screenH int, distance float64) (startX, startY, endX, endY int, err error) {
	if distance <= 0 {
		distance = 0.5
	}
	if distance > 1 {
		distance = 1
	}

	clamp := func(v, max int) int {
		if v < 0 {
			return 0
		}
		if max > 0 && v > max-1 {
			return max - 1
		}
		return v
	}
	cx, cy := b.Center()
	dx := int(float64(screenW) * distance)
	dy := int(float64(screenH) * distance)

	switch direction {
	case "up":
		return clamp(cx, screenW), clamp(cy, screenH), clamp(cx, screenW), clamp(cy-dy, screenH), nil
	case "down":
		return clamp(cx, screenW), clamp(cy, screenH), clamp(cx, screenW), clamp(cy+dy, screenH), nil
	case "left":
		return clamp(cx, screenW), clamp(cy, screenH), clamp(cx-dx, screenW), clamp(cy, screenH), nil
	case "right":
		return clamp(cx, screenW), clamp(cy, screenH), clamp(cx+dx, screenW), clamp(cy, screenH), nil
	default:
		return 0, 0, 0, 0, fmt.Errorf("invalid swipe direction: %q", direction)
	}
}
