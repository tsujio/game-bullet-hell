package main

import (
	"embed"
	"fmt"
	"image"
	"image/color"
	"log"
	"math"
	"math/rand"
	"os"
	"strconv"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/text"
	"github.com/hajimehoshi/ebiten/v2/vector"
	"github.com/tsujio/game-bullet-hell/touchutil"
	"github.com/tsujio/game-util/mathutil"
	"github.com/tsujio/game-util/resourceutil"
	"github.com/tsujio/go-bulletml"
)

const (
	gameName                 = "bullet-hell"
	screenWidth              = 640
	screenHeight             = 480
	playerR                  = 4
	playerBulletR            = 3
	enemyR                   = 4
	bulletR                  = 3
	playerHomeX, playerHomeY = screenWidth / 2, screenHeight * 4 / 5
)

//go:embed resources/*.ttf resources/*.xml
var resources embed.FS

var (
	fontL, fontM, fontS                                       = resourceutil.ForceLoadFont(resources, "resources/PressStart2P-Regular.ttf", nil)
	playerImg, playerBulletImg, enemyImg, bulletImg, emptyImg *ebiten.Image
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

	img := ebiten.NewImage(3, 3)
	img.Fill(color.White)
	emptyImg = img.SubImage(image.Rect(1, 1, 2, 2)).(*ebiten.Image)
}

type Enemy struct {
	pos, prevPos *mathutil.Vector2D
	r            float64
	hit          bool
	life         float64
	runner       bulletml.Runner
	game         *Game
}

func (e *Enemy) update() error {
	e.prevPos = e.pos.Clone()

	if e.hit {
		e.life -= 0.5
		e.hit = false
	}

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

	e.drawLife(dst)
}

func (e *Enemy) drawLife(dst *ebiten.Image) {
	var path vector.Path
	const r = 40.0

	path.MoveTo(float32(e.pos.X), float32(e.pos.Y-r))
	path.Arc(float32(e.pos.X), float32(e.pos.Y), float32(r), -math.Pi/2, float32(-math.Pi/2-2*math.Pi*e.life/100), vector.CounterClockwise)

	op := &vector.StrokeOptions{}
	op.Width = 5
	op.LineJoin = vector.LineJoinRound
	vs, is := path.AppendVerticesAndIndicesForStroke(nil, nil, op)

	for i := range vs {
		vs[i].SrcX = 1
		vs[i].SrcY = 1
		vs[i].ColorR = 0
		vs[i].ColorG = 0
		vs[i].ColorB = 0
		vs[i].ColorA = 0.5
	}

	opts := &ebiten.DrawTrianglesOptions{}
	dst.DrawTriangles(vs, is, emptyImg, opts)
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
	if b.pos.X-b.r > 0 && b.pos.X+b.r < screenWidth && b.pos.Y-b.r > 0 && b.pos.Y+b.r < screenHeight {
		opts := &ebiten.DrawImageOptions{}
		w, h := bulletImg.Size()
		opts.GeoM.Translate(b.pos.X-float64(w)/2, b.pos.Y-float64(h)/2)
		dst.DrawImage(bulletImg, opts)
	}
}

type Player struct {
	ticks           int
	pos, prevPos    *mathutil.Vector2D
	r               float64
	invincibleUntil int
	hit             bool
	game            *Game
}

func (p *Player) invincible() bool {
	return p.ticks <= p.invincibleUntil
}

func (p *Player) update() error {
	p.ticks++

	p.prevPos = p.pos.Clone()

	if p.hit {
		p.pos = mathutil.NewVector2D(playerHomeX, playerHomeY)
		p.invincibleUntil = p.ticks + 60*3
		p.hit = false
	}

	if len(p.game.touches) > 0 {
		t := p.game.touches[0]
		if prev := t.PreviousPosition(); prev != nil {
			if diff := t.Position().Sub(prev); diff.NormSq() > 0 {
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

	if !p.invincible() {
		if p.ticks%5 == 0 {
			for i := 0; i < 2; i++ {
				pos := p.pos.Clone()
				b := &PlayerBullet{
					pos: pos.Add(mathutil.NewVector2D(float64(10*(i*2-1)), -3)),
					r:   playerBulletR,
				}
				b.prevPos = b.pos
				p.game.playerBullets = append(p.game.playerBullets, b)
			}
		}
	}

	return nil
}

func (p *Player) draw(dst *ebiten.Image) {
	opts := &ebiten.DrawImageOptions{}
	w, h := playerImg.Size()
	opts.GeoM.Translate(p.pos.X-float64(w)/2, p.pos.Y-float64(h)/2)

	if p.invincible() && p.ticks/10%2 == 0 {
		opts.ColorScale.ScaleAlpha(0.2)
	}

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
	if b.pos.X-b.r > 0 && b.pos.X+b.r < screenWidth && b.pos.Y-b.r > 0 && b.pos.Y+b.r < screenHeight {
		opts := &ebiten.DrawImageOptions{}
		w, h := playerBulletImg.Size()
		opts.GeoM.Translate(b.pos.X-float64(w)/2, b.pos.Y-float64(h)/2)
		dst.DrawImage(playerBulletImg, opts)
	}
}

type GameMode int

const (
	GameModeTitle GameMode = iota
	GameModePlaying
	GameModeGameOver
)

type Game struct {
	touches            []touchutil.Touch
	random             *rand.Rand
	mode               GameMode
	ticksFromModeStart uint64
	player             *Player
	enemy              *Enemy
	bullets            []*Bullet
	playerBullets      []*PlayerBullet
}

func (g *Game) Update() error {
	g.touches = touchutil.AppendNewTouches(g.touches)
	for _, t := range g.touches {
		t.Update()
	}

	g.ticksFromModeStart++

	switch g.mode {
	case GameModeTitle:
		g.setNextMode(GameModePlaying)

	case GameModePlaying:
		if g.enemy.runner == nil {
			g.setBulletML("resources/barrage-1.xml")
		}

		if !g.player.invincible() {
			playerTopLeftX := math.Min(g.player.pos.X-g.player.r, g.player.prevPos.X-g.player.r)
			playerTopLeftY := math.Min(g.player.pos.Y-g.player.r, g.player.prevPos.Y-g.player.r)
			playerBottomRightX := math.Max(g.player.pos.X+g.player.r, g.player.prevPos.X+g.player.r)
			playerBottomRightY := math.Max(g.player.pos.Y+g.player.r, g.player.prevPos.Y+g.player.r)
			for _, b := range g.bullets {
				bulletTopLeftX := math.Min(b.pos.X-b.r, b.prevPos.X-b.r)
				bulletTopLeftY := math.Min(b.pos.Y-b.r, b.prevPos.Y-b.r)
				bulletBottomRightX := math.Max(b.pos.X+b.r, b.prevPos.X+b.r)
				bulletBottomRightY := math.Max(b.pos.Y+b.r, b.prevPos.Y+b.r)

				if bulletTopLeftX > playerBottomRightX ||
					bulletTopLeftY > playerBottomRightY ||
					bulletBottomRightX < playerTopLeftX ||
					bulletBottomRightY < playerTopLeftY {
					continue
				}

				if mathutil.CapsulesCollide(
					g.player.pos, g.player.prevPos.Sub(g.player.pos), g.player.r,
					b.pos, b.prevPos.Sub(b.pos), b.r,
				) {
					b.hit = true
					g.player.hit = true

					g.touches = nil

					break
				}
			}
		}

		for _, b := range g.playerBullets {
			if mathutil.CapsulesCollide(
				g.enemy.pos, g.enemy.prevPos.Sub(g.enemy.pos), g.enemy.r,
				b.pos, b.prevPos.Sub(b.pos), b.r,
			) {
				b.hit = true
				g.enemy.hit = true

				g.enemy.life -= 0.5
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
				b.prevPos.X-b.r > 0 && b.prevPos.X+b.r < screenWidth && b.prevPos.Y-b.r > 0 && b.prevPos.Y+b.r < screenHeight {
				_bullets = append(_bullets, b)
			}
		}
		g.bullets = _bullets

		_playerBullets := g.playerBullets[:0]
		for _, b := range g.playerBullets {
			if !b.hit &&
				b.prevPos.X-b.r > 0 && b.prevPos.X+b.r < screenWidth && b.prevPos.Y-b.r > 0 && b.prevPos.Y+b.r < screenHeight {
				_playerBullets = append(_playerBullets, b)
			}
		}
		g.playerBullets = _playerBullets

		if g.enemy.life <= 0 {
			g.setNextMode(GameModeGameOver)
		}

	case GameModeGameOver:
		if g.ticksFromModeStart > 60 && len(g.touches) > 0 && g.touches[0].IsJustTouched() {
			g.initialize()
		}
	}

	_touches := g.touches[:0]
	for _, t := range g.touches {
		if !t.IsJustReleased() {
			_touches = append(_touches, t)
		}
	}
	g.touches = _touches

	return nil
}

func (g *Game) drawGameOverText(screen *ebiten.Image) {
	var gameOverTexts []string
	if g.enemy.life <= 0 {
		gameOverTexts = []string{"GAME CLEAR"}
	} else {
		gameOverTexts = []string{"GAME OVER"}
	}
	for i, s := range gameOverTexts {
		text.Draw(screen, s, fontL.Face, screenWidth/2-len(s)*int(fontL.FaceOptions.Size)/2, 170+i*int(fontL.FaceOptions.Size*1.8), color.Black)
	}

	scoreText := []string{"YOUR SCORE IS", fmt.Sprintf("%d!", 100)}
	for i, s := range scoreText {
		text.Draw(screen, s, fontM.Face, screenWidth/2-len(s)*int(fontM.FaceOptions.Size)/2, 230+i*int(fontM.FaceOptions.Size*1.8), color.Black)
	}
}

func (g *Game) Draw(screen *ebiten.Image) {
	screen.Fill(color.White)

	switch g.mode {
	case GameModeTitle:
	case GameModePlaying:
		g.player.draw(screen)

		for _, b := range g.bullets {
			b.draw(screen)
		}

		g.enemy.draw(screen)

		for _, b := range g.playerBullets {
			b.draw(screen)
		}
	case GameModeGameOver:
		g.player.draw(screen)

		for _, b := range g.bullets {
			b.draw(screen)
		}

		g.enemy.draw(screen)

		for _, b := range g.playerBullets {
			b.draw(screen)
		}

		g.drawGameOverText(screen)
	}

	ebitenutil.DebugPrint(screen, fmt.Sprintf("%.1ffps", ebiten.ActualFPS()))
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

func (g *Game) setNextMode(mode GameMode) {
	g.mode = mode
	g.ticksFromModeStart = 0
}

func (g *Game) initialize() {
	g.touches = nil

	playerPos := mathutil.NewVector2D(playerHomeX, playerHomeY)
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
		life:    100,
		game:    g,
	}

	g.bullets = nil
	g.playerBullets = nil

	g.setNextMode(GameModeTitle)
}

func main() {
	var seed int64
	if s, err := strconv.Atoi(os.Getenv("GAME_RAND_SEED")); err == nil {
		seed = int64(s)
	}

	ebiten.SetWindowSize(screenWidth, screenHeight)
	ebiten.SetWindowTitle("Bullet Hell")

	game := &Game{
		random:             rand.New(rand.NewSource(seed)),
		ticksFromModeStart: 0,
	}
	game.initialize()

	if err := ebiten.RunGame(game); err != nil {
		log.Fatal(err)
	}
}
