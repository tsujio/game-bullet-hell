package main

import (
	"embed"
	"fmt"
	"image/color"
	"log"
	"math/rand"
	"os"
	"strconv"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/vector"
	"github.com/samber/lo"
	"github.com/tsujio/game-bullet-hell/touchutil"
	"github.com/tsujio/game-util/mathutil"
	"github.com/tsujio/go-bulletml"
)

const (
	gameName      = "bullet-hell"
	screenWidth   = 640
	screenHeight  = 480
	playerR       = 4
	playerBulletR = 3
	enemyR        = 4
	bulletR       = 3
)

//go:embed resources/*.xml
var resources embed.FS

var (
	playerImg, playerBulletImg, enemyImg, bulletImg *ebiten.Image
)

func init() {
	playerImg = ebiten.NewImage(playerR*2, playerR*2)
	vector.DrawFilledCircle(playerImg, playerR, playerR, playerR, color.Black, true)

	playerBulletImg = ebiten.NewImage(playerBulletR*2, playerBulletR*2)
	vector.DrawFilledCircle(playerBulletImg, playerBulletR, playerBulletR, playerBulletR, color.Black, true)

	enemyImg = ebiten.NewImage(enemyR*2, enemyR*2)
	vector.DrawFilledCircle(enemyImg, enemyR, enemyR, enemyR, color.Black, true)

	bulletImg = ebiten.NewImage(bulletR*2, bulletR*2)
	vector.DrawFilledCircle(bulletImg, bulletR, bulletR, bulletR, color.Black, true)
}

type Enemy struct {
	pos, prevPos *mathutil.Vector2D
	r            float64
	runner       bulletml.Runner
	game         *Game
}

func (e *Enemy) update() error {
	e.prevPos = e.pos.Clone()

	if e.runner != nil {
		if err := e.runner.Update(); err != nil {
			return err
		}
	}

	return nil
}

func (e *Enemy) draw(dst *ebiten.Image) {
	opts := &ebiten.DrawImageOptions{}
	w, h := enemyImg.Size()
	opts.GeoM.Translate(e.pos.X-float64(w)/2, e.pos.Y-float64(h)/2)
	dst.DrawImage(enemyImg, opts)
}

type Bullet struct {
	pos, prevPos *mathutil.Vector2D
	r            float64
	hit          bool
	runner       bulletml.BulletRunner
	game         *Game
}

func (b *Bullet) update() error {
	b.prevPos = b.pos.Clone()

	if err := b.runner.Update(); err != nil {
		return err
	}

	b.pos.X, b.pos.Y = b.runner.Position()

	return nil
}

func (b *Bullet) draw(dst *ebiten.Image) {
	opts := &ebiten.DrawImageOptions{}
	w, h := bulletImg.Size()
	opts.GeoM.Translate(b.pos.X-float64(w)/2, b.pos.Y-float64(h)/2)
	dst.DrawImage(bulletImg, opts)
}

type Player struct {
	pos, prevPos *mathutil.Vector2D
	r            float64
	game         *Game
}

func (p *Player) update() error {
	p.prevPos = p.pos.Clone()

	if len(p.game.touches) > 0 {
		if p.game.touches[0].IsJustReleased() {
			p.game.touchPosHistory = make([]*mathutil.Vector2D, 60)
		} else {
			curr := p.game.touches[0].Position()
			p.game.touchPosHistory[p.game.ticks%uint64(len(p.game.touchPosHistory))] = curr
			if prev := p.game.touchPosHistory[(p.game.ticks-1)%uint64(len(p.game.touchPosHistory))]; prev != nil {
				diff := curr.Sub(prev)
				if norm := diff.Norm(); norm > 0 {
					p.pos = p.pos.Add(diff)

					if p.pos.X < 0 {
						p.pos.X = 0
					}
					if p.pos.X > screenWidth {
						p.pos.X = screenWidth
					}
					if p.pos.Y < 0 {
						p.pos.Y = 0
					}
					if p.pos.Y > screenHeight {
						p.pos.Y = screenHeight
					}
				}
			}
		}
	}

	if p.game.ticks%5 == 0 {
		b := &PlayerBullet{
			pos:     p.pos.Clone(),
			prevPos: p.pos.Clone(),
			r:       playerBulletR,
		}
		p.game.playerBullets = append(p.game.playerBullets, b)
	}

	return nil
}

func (p *Player) draw(dst *ebiten.Image) {
	opts := &ebiten.DrawImageOptions{}
	w, h := playerImg.Size()
	opts.GeoM.Translate(p.pos.X-float64(w)/2, p.pos.Y-float64(h)/2)
	dst.DrawImage(playerImg, opts)
}

type PlayerBullet struct {
	pos, prevPos *mathutil.Vector2D
	r            float64
	hit          bool
}

func (b *PlayerBullet) update() error {
	b.prevPos = b.pos.Clone()

	b.pos = b.pos.Add(mathutil.NewVector2D(0, -10))

	return nil
}

func (b *PlayerBullet) draw(dst *ebiten.Image) {
	opts := &ebiten.DrawImageOptions{}
	w, h := playerBulletImg.Size()
	opts.GeoM.Translate(b.pos.X-float64(w)/2, b.pos.Y-float64(h)/2)
	dst.DrawImage(playerBulletImg, opts)
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
	enemy           *Enemy
	bullets         []*Bullet
	playerBullets   []*PlayerBullet
}

func (g *Game) Update() error {
	g.touches = touchutil.AppendNewTouches(g.touches)

	g.ticks++

	if g.enemy.runner == nil {
		g.setBulletML("resources/barrage-1.xml")
	}

	for _, b := range g.bullets {
		if mathutil.CapsulesCollide(
			g.player.pos, g.player.prevPos.Sub(g.player.pos), g.player.r,
			b.pos, b.prevPos.Sub(b.pos), b.r,
		) {
			b.hit = true

			g.player.pos = mathutil.NewVector2D(
				screenWidth/2,
				screenHeight*2/3,
			)

			break
		}
	}

	for _, b := range g.playerBullets {
		if mathutil.CapsulesCollide(
			g.enemy.pos, g.enemy.prevPos.Sub(g.enemy.pos), g.enemy.r,
			b.pos, b.prevPos.Sub(b.pos), b.r,
		) {
			b.hit = true
		}
	}

	if err := g.player.update(); err != nil {
		return err
	}

	if err := g.enemy.update(); err != nil {
		return err
	}

	for i, n := 0, len(g.bullets); i < n; i++ {
		if err := g.bullets[i].update(); err != nil {
			return err
		}
	}

	for i, n := 0, len(g.playerBullets); i < n; i++ {
		if err := g.playerBullets[i].update(); err != nil {
			return err
		}
	}

	_bullets := g.bullets[:0]
	for _, b := range g.bullets {
		if !b.hit &&
			!b.runner.Vanished() &&
			b.pos.Sub(mathutil.NewVector2D(screenWidth/2, screenHeight/2)).Norm() < 500 {
			_bullets = append(_bullets, b)
		}
	}
	g.bullets = _bullets

	_playerBullets := g.playerBullets[:0]
	for _, b := range g.playerBullets {
		if !b.hit && b.pos.Sub(mathutil.NewVector2D(screenWidth/2, screenHeight/2)).Norm() < 500 {
			_playerBullets = append(_playerBullets, b)
		}
	}
	g.playerBullets = _playerBullets

	g.touches = lo.Filter(g.touches, func(t touchutil.Touch, _ int) bool {
		return !t.IsJustReleased()
	})

	return nil
}

func (g *Game) Draw(screen *ebiten.Image) {
	screen.Fill(color.White)

	g.enemy.draw(screen)

	g.player.draw(screen)

	for _, b := range g.bullets {
		b.draw(screen)
	}

	for _, b := range g.playerBullets {
		b.draw(screen)
	}

	ebitenutil.DebugPrint(screen, fmt.Sprintf("%.1f", ebiten.ActualFPS()))
}

func (g *Game) Layout(outsideWidth, outsideHeight int) (int, int) {
	return screenWidth, screenHeight
}

func (g *Game) setBulletML(path string) error {
	f, err := resources.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	bml, err := bulletml.Load(f)
	if err != nil {
		return err
	}

	opts := &bulletml.NewRunnerOptions{
		OnBulletFired: func(br bulletml.BulletRunner, fc *bulletml.FireContext) {
			x, y := br.Position()
			b := &Bullet{
				pos:     mathutil.NewVector2D(x, y),
				prevPos: mathutil.NewVector2D(x, y),
				r:       bulletR,
				runner:  br,
				game:    g,
			}
			g.bullets = append(g.bullets, b)
		},
		CurrentShootPosition: func() (float64, float64) {
			return g.enemy.pos.X, g.enemy.pos.Y
		},
		CurrentTargetPosition: func() (float64, float64) {
			return g.player.pos.X, g.player.pos.Y
		},
	}

	runner, err := bulletml.NewRunner(bml, opts)
	if err != nil {
		return err
	}

	g.enemy.runner = runner

	return nil
}

func (g *Game) initialize() {
	g.touches = nil
	g.touchPosHistory = make([]*mathutil.Vector2D, 60)

	playerPos := mathutil.NewVector2D(
		screenWidth/2,
		screenHeight*4/5,
	)
	g.player = &Player{
		pos:     playerPos,
		prevPos: playerPos,
		r:       playerR,
		game:    g,
	}

	enemyPos := mathutil.NewVector2D(
		screenWidth/2,
		screenHeight*1/5,
	)
	g.enemy = &Enemy{
		pos:     enemyPos,
		prevPos: enemyPos,
		r:       enemyR,
		game:    g,
	}

	g.bullets = nil
	g.playerBullets = nil
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
