package main

import (
	"image/color"
	"log"
	"math/rand"
	"os"
	"strconv"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
	"github.com/samber/lo"
	"github.com/tsujio/game-util/mathutil"
	"github.com/tsujio/game-util/touchutil"
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

type Game struct {
	touchContext *touchutil.TouchContext
	random       *rand.Rand
	ticks        uint64
	touchBasePos *mathutil.Vector2D
	player       *Player
	bullets      []*Bullet
	enemies      []*Enemy
}

func (g *Game) Update() error {
	g.touchContext.Update()

	g.ticks++

	if g.touchContext.IsJustTouched() {
		p := g.touchContext.GetTouchPosition()
		g.touchBasePos = mathutil.NewVector2D(float64(p.X), float64(p.Y))
	}

	if g.touchContext.IsBeingTouched() {
		p := g.touchContext.GetTouchPosition()
		curr := mathutil.NewVector2D(float64(p.X), float64(p.Y))
		diff := curr.Sub(g.touchBasePos)
		if norm := diff.Norm(); norm > 0 {
			d := diff.Normalize().Mul(0.8)
			if norm > 20 {
				d = d.Mul(2.0)
			}
			g.player.pos = g.player.pos.Add(d)
		}
	} else {
		g.touchBasePos = nil
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

	g.bullets = lo.Map(g.bullets, func(b *Bullet, _ int) *Bullet {
		b.pos = b.pos.Add(b.v)
		return b
	})

	g.bullets = lo.Filter(g.bullets, func(b *Bullet, _ int) bool {
		return b.pos.Sub(mathutil.NewVector2D(screenWidth/2, screenHeight/2)).Norm() < 500
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

	if g.touchBasePos != nil {
		vector.StrokeCircle(screen, float32(g.touchBasePos.X), float32(g.touchBasePos.Y), 20, 2, color.Black, true)
		vector.StrokeCircle(screen, float32(g.touchBasePos.X), float32(g.touchBasePos.Y), 40, 1, color.Black, true)
	}
}

func (g *Game) Layout(outsideWidth, outsideHeight int) (int, int) {
	return screenWidth, screenHeight
}

func (g *Game) initialize() {
	g.touchBasePos = nil
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
		touchContext: touchutil.CreateTouchContext(),
		random:       rand.New(rand.NewSource(seed)),
		ticks:        0,
	}
	game.initialize()

	if err := ebiten.RunGame(game); err != nil {
		log.Fatal(err)
	}
}
