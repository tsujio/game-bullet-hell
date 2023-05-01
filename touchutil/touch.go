package touchutil

import (
	"fmt"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/tsujio/game-util/mathutil"
)

var (
	justScreenTouchedIDs = make([]ebiten.TouchID, 0)
)

func AppendNewTouches(touches []Touch) []Touch {
	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		found := false
		for _, t := range touches {
			if m, ok := t.(*mouseButtonPress); ok && m.id == ebiten.MouseButtonLeft {
				found = true
				break
			}
		}
		if !found {
			touches = append(touches, &mouseButtonPress{
				id: ebiten.MouseButtonLeft,
			})
		}
	}

	justScreenTouchedIDs = inpututil.AppendJustPressedTouchIDs(justScreenTouchedIDs[:0])
	for _, id := range justScreenTouchedIDs {
		found := false
		for _, t := range touches {
			if s, ok := t.(*screenTouch); ok && s.id == id {
				found = true
				break
			}
		}
		if !found {
			touches = append(touches, &screenTouch{
				id: id,
			})
		}
	}

	return touches
}

type Touch interface {
	ID() []byte
	IsJustTouched() bool
	IsJustReleased() bool
	Position() *mathutil.Vector2D
}

type mouseButtonPress struct {
	id ebiten.MouseButton
}

func (m *mouseButtonPress) ID() []byte {
	return []byte(fmt.Sprintf("mouse-%d", m.id))
}

func (m *mouseButtonPress) IsJustTouched() bool {
	return inpututil.IsMouseButtonJustPressed(m.id)
}

func (m *mouseButtonPress) IsJustReleased() bool {
	return inpututil.IsMouseButtonJustReleased(m.id)
}

func (m *mouseButtonPress) Position() *mathutil.Vector2D {
	x, y := ebiten.CursorPosition()
	return mathutil.NewVector2D(float64(x), float64(y))
}

type screenTouch struct {
	id ebiten.TouchID
}

func (s *screenTouch) ID() []byte {
	return []byte(fmt.Sprintf("screen-%d", s.id))
}

func (s *screenTouch) IsJustTouched() bool {
	for _, id := range justScreenTouchedIDs {
		if id == s.id {
			return true
		}
	}
	return false
}

func (s *screenTouch) IsJustReleased() bool {
	return inpututil.IsTouchJustReleased(s.id)
}

func (s *screenTouch) Position() *mathutil.Vector2D {
	var x, y int
	if s.IsJustReleased() {
		x, y = inpututil.TouchPositionInPreviousTick(s.id)
	} else {
		x, y = ebiten.TouchPosition(s.id)
	}
	return mathutil.NewVector2D(float64(x), float64(y))
}
