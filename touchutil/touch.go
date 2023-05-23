package touchutil

import (
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/tsujio/game-util/mathutil"
)

var (
	justScreenTouchedIDs = make([]ebiten.TouchID, 0)
)

func AppendNewTouches(touches []Touch) []Touch {
	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		touches = append(touches, &mouseButtonPress{
			id: ebiten.MouseButtonLeft,
		})
	}

	justScreenTouchedIDs = inpututil.AppendJustPressedTouchIDs(justScreenTouchedIDs[:0])
	for _, id := range justScreenTouchedIDs {
		touches = append(touches, &screenTouch{
			id: id,
		})
	}

	return touches
}

type TouchType int

const (
	TouchTypeMouseButtonPress = iota
	TouchTypeScreenTouch
)

type TouchID struct {
	touchType TouchType
	apiID     any
}

type Touch interface {
	Update()
	ID() TouchID
	IsJustTouched() bool
	IsJustReleased() bool
	Position() *mathutil.Vector2D
	PreviousPosition() *mathutil.Vector2D
}

type mouseButtonPress struct {
	id           ebiten.MouseButton
	pos, prevPos *mathutil.Vector2D
}

func (m *mouseButtonPress) Update() {
	if m.pos != nil {
		m.prevPos = m.pos.Clone()
	}
	x, y := ebiten.CursorPosition()
	m.pos = mathutil.NewVector2D(float64(x), float64(y))
}

func (m *mouseButtonPress) ID() TouchID {
	return TouchID{touchType: TouchTypeMouseButtonPress, apiID: m.id}
}

func (m *mouseButtonPress) IsJustTouched() bool {
	return inpututil.IsMouseButtonJustPressed(m.id)
}

func (m *mouseButtonPress) IsJustReleased() bool {
	return inpututil.IsMouseButtonJustReleased(m.id)
}

func (m *mouseButtonPress) Position() *mathutil.Vector2D {
	return m.pos
}

func (m *mouseButtonPress) PreviousPosition() *mathutil.Vector2D {
	return m.prevPos
}

type screenTouch struct {
	id           ebiten.TouchID
	pos, prevPos *mathutil.Vector2D
}

func (s *screenTouch) Update() {
	if s.pos != nil {
		s.prevPos = s.pos.Clone()
	}
	var x, y int
	if s.IsJustReleased() {
		x, y = inpututil.TouchPositionInPreviousTick(s.id)
	} else {
		x, y = ebiten.TouchPosition(s.id)
	}
	s.pos = mathutil.NewVector2D(float64(x), float64(y))
}

func (s *screenTouch) ID() TouchID {
	return TouchID{touchType: TouchTypeScreenTouch, apiID: s.id}
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
	return s.pos
}

func (s *screenTouch) PreviousPosition() *mathutil.Vector2D {
	return s.prevPos
}
