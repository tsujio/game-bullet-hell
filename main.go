package main

import (
	"fmt"
	"image/color"
	"log"
	"math"
	"math/rand"
	"os"
	"strconv"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/vector"
	"github.com/samber/lo"
	"github.com/tsujio/game-bullet-hell/touchutil"
	"github.com/tsujio/game-util/mathutil"
)

const (
	gameName     = "bullet-hell"
	screenWidth  = 640
	screenHeight = 480
)

type Enemy struct {
	pos *mathutil.Vector2D
	r   float64
}

func (e *Enemy) Draw(dst *ebiten.Image) {
	vector.DrawFilledRect(dst, float32(e.pos.X-e.r), float32(e.pos.Y-e.r), float32(e.r)*2, float32(e.r)*2, color.Black, true)
}

type Bullet struct {
	pos *mathutil.Vector2D
	r   float64
	v   *mathutil.Vector2D
	hit bool
}

func (b *Bullet) Draw(dst *ebiten.Image) {
	vector.DrawFilledCircle(dst, float32(b.pos.X), float32(b.pos.Y), float32(b.r), color.Black, true)
}

type Player struct {
	pos *mathutil.Vector2D
	r   float64
}

func (p *Player) Draw(dst *ebiten.Image) {
	vector.DrawFilledCircle(dst, float32(p.pos.X), float32(p.pos.Y), float32(p.r), color.Black, true)
}

type Touch struct {
	id  []byte
	pos *mathutil.Vector2D
}

type Game struct {
	touches         []touchutil.Touch
	random          *rand.Rand
	ticks           uint64
	touchPosHistory []*mathutil.Vector2D
	player          *Player
	bullets         []*Bullet
	enemies         []*Enemy
}

func (g *Game) Update() error {
	g.touches = touchutil.AppendNewTouches(g.touches)

	g.ticks++

	if len(g.touches) > 0 {
		if g.touches[0].IsJustReleased() {
			g.touchPosHistory = make([]*mathutil.Vector2D, 60)
		} else {
			curr := g.touches[0].Position()
			g.touchPosHistory[g.ticks%uint64(len(g.touchPosHistory))] = curr
			if prev := g.touchPosHistory[(g.ticks-1)%uint64(len(g.touchPosHistory))]; prev != nil {
				diff := curr.Sub(prev)
				if norm := diff.Norm(); norm > 0 {
					g.player.pos = g.player.pos.Add(diff)

					if g.player.pos.X < 0 {
						g.player.pos.X = 0
					}
					if g.player.pos.X > screenWidth {
						g.player.pos.X = screenWidth
					}
					if g.player.pos.Y < 0 {
						g.player.pos.Y = 0
					}
					if g.player.pos.Y > screenHeight {
						g.player.pos.Y = screenHeight
					}
				}
			}
		}
	}

	if g.ticks == 1 {
		for i := 0; i < 2; i++ {
			e := &Enemy{
				pos: mathutil.NewVector2D(float64(screenWidth/2+150.0*(i*2-1)), 50),
				r:   10,
			}
			g.enemies = append(g.enemies, e)
		}
	}

	if g.ticks%15 == 0 {
		for _, e := range g.enemies {
			b := &Bullet{
				pos: e.pos.Clone(),
				r:   3,
				v:   g.player.pos.Sub(e.pos).Normalize().Mul(3),
			}
			g.bullets = append(g.bullets, b)
		}
	}

	for _, b := range g.bullets {
		if math.Pow(b.pos.X-g.player.pos.X, 2)+math.Pow(b.pos.Y-g.player.pos.Y, 2) < math.Pow(b.r+g.player.r, 2) {
			b.hit = true

			g.player.pos = mathutil.NewVector2D(
				screenWidth/2,
				screenHeight*2/3,
			)

			break
		}
	}

	g.bullets = lo.Map(g.bullets, func(b *Bullet, _ int) *Bullet {
		b.pos = b.pos.Add(b.v)
		return b
	})

	g.bullets = lo.Filter(g.bullets, func(b *Bullet, _ int) bool {
		return !b.hit && b.pos.Sub(mathutil.NewVector2D(screenWidth/2, screenHeight/2)).Norm() < 500
	})

	g.touches = lo.Filter(g.touches, func(t touchutil.Touch, _ int) bool {
		return !t.IsJustReleased()
	})

	return nil
}

func (g *Game) Draw(screen *ebiten.Image) {
	screen.Fill(color.White)

	for _, b := range g.bullets {
		b.Draw(screen)
	}

	for _, e := range g.enemies {
		e.Draw(screen)
	}

	g.player.Draw(screen)

	ebitenutil.DebugPrint(screen, fmt.Sprintf("%.1f", ebiten.ActualFPS()))
}

func (g *Game) Layout(outsideWidth, outsideHeight int) (int, int) {
	return screenWidth, screenHeight
}

func (g *Game) initialize() {
	g.touches = nil
	g.touchPosHistory = make([]*mathutil.Vector2D, 60)
	g.player = &Player{
		pos: mathutil.NewVector2D(
			screenWidth/2,
			screenHeight*2/3,
		),
		r: 3.0,
	}
	g.bullets = nil
	g.enemies = nil
}

func main() {
	var seed int64
	if s, err := strconv.Atoi(os.Getenv("GAME_RAND_SEED")); err == nil {
		seed = int64(s)
	}

	ebiten.SetWindowSize(screenWidth, screenHeight)
	ebiten.SetWindowTitle("Bullet Hell")

	game := &Game{
		random: rand.New(rand.NewSource(seed)),
		ticks:  0,
	}
	game.initialize()

	if err := ebiten.RunGame(game); err != nil {
		log.Fatal(err)
	}
}
