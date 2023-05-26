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
	playerGrazeR             = 8
	playerBulletR            = 3
	enemyR                   = 20
	bulletR                  = 3
	playerHomeX, playerHomeY = screenWidth / 2, screenHeight * 4 / 5
	enemyHomeX, enemyHomeY   = screenWidth / 2, screenHeight * 1 / 5
	playerInitialLife        = 6
	bulletMLGain             = 10000
	failureGain              = -1000
	grazeGain                = 10
)

//go:embed resources/*.ttf resources/*.xml
var resources embed.FS

var (
	fontL, fontM, fontS = resourceutil.ForceLoadFont(resources, "resources/PressStart2P-Regular.ttf", nil)
	_, _, fontSS        = resourceutil.ForceLoadFont(resources, "resources/PressStart2P-Regular.ttf", &resourceutil.LoadFontOption{
		SmallSize: 8,
	})
	emptyImg,
	playerImg,
	playerBulletImg,
	enemyImg,
	bulletImg *ebiten.Image
	playerLifeImgs []*ebiten.Image
	flashImg       *ebiten.Image
	bulletMLs      []*bulletml.BulletML
)

func init() {
	img := ebiten.NewImage(3, 3)
	img.Fill(color.White)
	emptyImg = img.SubImage(image.Rect(1, 1, 2, 2)).(*ebiten.Image)

	playerImg = ebiten.NewImage(playerR*2, playerR*2)
	vector.DrawFilledCircle(playerImg, playerR, playerR, playerR, color.RGBA{0xff, 0, 0, 0xff}, true)

	playerBulletImg = ebiten.NewImage(playerBulletR*2, playerBulletR*2)
	vector.DrawFilledCircle(playerBulletImg, playerBulletR, playerBulletR, playerBulletR, color.Black, true)

	enemyImg = ebiten.NewImage(enemyR*2, enemyR*2)
	vector.DrawFilledRect(enemyImg, 0, 0, enemyR*2, enemyR*2, color.Black, true)

	bulletImg = ebiten.NewImage(bulletR*2, bulletR*2)
	vector.DrawFilledCircle(bulletImg, bulletR, bulletR, bulletR, color.Black, true)

	for life := 1; life <= playerInitialLife; life++ {
		img = ebiten.NewImage(70, 70)
		w, _ := img.Size()
		for i, n := 0, life-1; i < n; i++ {
			x := float32(float64(w)/2 + (float64(w)/2-2)*math.Cos(math.Pi*2*float64(i)/float64(n)-math.Pi/2))
			y := float32(float64(w)/2 + (float64(w)/2-2)*math.Sin(math.Pi*2*float64(i)/float64(n)-math.Pi/2))
			vector.DrawFilledCircle(img, x, y, 2, color.RGBA{0, 0, 0, 0x70}, true)
		}
		playerLifeImgs = append(playerLifeImgs, img)
	}

	flashImg = ebiten.NewImage(500, 500)
	vector.DrawFilledCircle(flashImg, 250, 250, 250, color.White, true)

	for _, p := range []string{"barrage-1.xml", "barrage-1.xml"} {
		f, err := resources.Open("resources/" + p)
		if err != nil {
			panic(err)
		}
		defer f.Close()

		bml, err := bulletml.Load(f)
		if err != nil {
			panic(err)
		}

		bulletMLs = append(bulletMLs, bml)
	}
}

func isIn[T comparable](v T, values ...T) bool {
	for _, val := range values {
		if v == val {
			return true
		}
	}
	return false
}

type EnemyState int

const (
	EnemyStateWaiting = iota
	EnemyStateRunning
	EnemyStateFlashing
	EnemyStateExploded
)

type Enemy struct {
	ticks               int
	pos, prevPos        *mathutil.Vector2D
	r                   float64
	state               EnemyState
	hit                 bool
	life                float64
	bulletMLIndex       int
	startNextBulletMLAt int
	explodeAt           int
	runner              bulletml.BulletRunner
	game                *Game
}

func (e *Enemy) update() error {
	e.prevPos = e.pos.Clone()

	switch e.state {
	case EnemyStateWaiting:
		home := mathutil.NewVector2D(enemyHomeX, enemyHomeY)
		e.pos = e.pos.Add(home.Sub(e.pos).Div(60))

		if e.ticks == e.startNextBulletMLAt {
			e.game.setBulletML(e.bulletMLIndex)
			e.state = EnemyStateRunning
		}
	case EnemyStateRunning:
		if err := e.runner.Update(); err != nil {
			return err
		}
		e.pos.X, e.pos.Y = e.runner.Position()

		if e.hit {
			e.life -= 0.5
		}

		if e.life <= 0 {
			for _, b := range e.game.bullets {
				f := &FlashEffect{
					pos:   b.pos.Clone(),
					r:     10,
					color: color.Gray{0x70},
					until: 25,
				}
				e.game.flashEffects = append(e.game.flashEffects, f)
			}

			e.game.score += int(math.Max(float64(bulletMLGain+e.game.failuresInBulletMLRunning*failureGain), 0))
			e.game.failuresInBulletMLRunning = 0

			e.runner = nil
			e.game.bullets = nil
			e.bulletMLIndex++

			if e.bulletMLIndex < len(bulletMLs) {
				e.startNextBulletMLAt = e.ticks + 180
				e.life = 100
				e.state = EnemyStateWaiting
			} else {
				e.explodeAt = e.ticks + 120
				e.state = EnemyStateFlashing
			}
		}
	case EnemyStateFlashing:
		if e.ticks%15 == 0 {
			f := &FlashEffect{
				pos:   e.pos.Clone().Add(mathutil.NewVector2D(50*e.game.random.Float64()-25, 50*e.game.random.Float64()-25)),
				r:     60,
				color: color.Black,
				until: 30,
			}
			e.game.flashEffects = append(e.game.flashEffects, f)
		}

		if e.ticks == e.explodeAt {
			for i := 0; i < 50; i++ {
				s := 2 + 4*e.game.random.Float64()
				d := math.Pi * 2 * e.game.random.Float64()
				f := &EnemyFragment{
					pos: e.pos.Clone(),
					v:   mathutil.NewVector2D(s*math.Cos(d), s*math.Sin(d)),
				}
				e.game.enemyFragments = append(e.game.enemyFragments, f)
			}

			e.state = EnemyStateExploded
		}
	case EnemyStateExploded:
	}

	if e.hit {
		e.hit = false
	}

	e.ticks++

	return nil
}

func (e *Enemy) draw(dst *ebiten.Image) {
	if isIn(e.state, EnemyStateWaiting, EnemyStateRunning, EnemyStateFlashing) {
		opts := &ebiten.DrawImageOptions{}
		w, h := enemyImg.Size()
		opts.GeoM.Translate(-float64(w)/2, -float64(h)/2)
		opts.GeoM.Rotate(float64(e.ticks) * math.Pi / 30)
		opts.GeoM.Translate(e.pos.X, e.pos.Y)
		dst.DrawImage(enemyImg, opts)

		if e.life > 0 {
			e.drawLife(dst)
		}
	}
}

func (e *Enemy) drawLife(dst *ebiten.Image) {
	var path vector.Path
	const r = 60.0

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
		vs[i].ColorA = 0.3
	}

	opts := &ebiten.DrawTrianglesOptions{}
	dst.DrawTriangles(vs, is, emptyImg, opts)
}

type Bullet struct {
	pos, prevPos *mathutil.Vector2D
	r            float64
	hit          bool
	grazed       bool
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
	grazeR          float64
	invincibleUntil int
	hit             bool
	life            int
	game            *Game
}

func (p *Player) invincible() bool {
	return p.ticks <= p.invincibleUntil
}

func (p *Player) update() error {
	p.prevPos = p.pos.Clone()

	if p.hit {
		p.pos = mathutil.NewVector2D(playerHomeX, playerHomeY)
		p.invincibleUntil = p.ticks + 60*3
		p.life--
		p.game.failuresInBulletMLRunning++
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

	if !p.invincible() && p.life > 0 {
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

	p.ticks++

	return nil
}

func (p *Player) draw(dst *ebiten.Image) {
	if p.life > 0 {
		opts := &ebiten.DrawImageOptions{}
		w, h := playerImg.Size()
		opts.GeoM.Translate(p.pos.X-float64(w)/2, p.pos.Y-float64(h)/2)

		if p.invincible() && p.ticks/10%2 == 0 {
			opts.ColorScale.ScaleAlpha(0.2)
		}

		dst.DrawImage(playerImg, opts)

		p.drawLife(dst)
	}
}

func (p *Player) drawLife(dst *ebiten.Image) {
	if p.life > 0 {
		img := playerLifeImgs[p.life-1]

		opts := &ebiten.DrawImageOptions{}
		w, h := img.Size()
		opts.GeoM.Translate(-float64(w)/2, -float64(h)/2)
		opts.GeoM.Rotate(float64(p.ticks) * math.Pi / 30)
		opts.GeoM.Translate(p.pos.X, p.pos.Y)

		if p.invincible() && p.ticks/10%2 == 0 {
			opts.ColorScale.ScaleAlpha(0.2)
		}

		dst.DrawImage(img, opts)
	}
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
		opts.ColorScale.ScaleAlpha(0.3)
		dst.DrawImage(playerBulletImg, opts)
	}
}

type FlashEffect struct {
	ticks    int
	pos      *mathutil.Vector2D
	r        float64
	v        *mathutil.Vector2D
	color    color.Color
	until    int
	finished bool
}

func (e *FlashEffect) update() error {
	e.ticks++

	if e.v != nil {
		e.pos = e.pos.Add(e.v)
	}

	if e.ticks >= e.until {
		e.finished = true
	}

	return nil
}

func (e *FlashEffect) draw(dst *ebiten.Image) {
	rad := e.r * float64(e.ticks) / float64(e.until)
	r, g, b, a := e.color.RGBA()
	a = uint32(float64(a) * (1 - float64(e.ticks)/float64(e.until)))

	opts := &ebiten.DrawImageOptions{}
	w, h := flashImg.Size()
	opts.GeoM.Translate(-float64(w)/2, -float64(h)/2)
	opts.GeoM.Scale(rad*2/float64(w), rad*2/float64(h))
	opts.GeoM.Translate(e.pos.X, e.pos.Y)
	opts.ColorScale.Scale(float32(r)/0xffff, float32(g)/0xffff, float32(b)/0xffff, float32(a)/0xffff)
	dst.DrawImage(flashImg, opts)
}

type EnemyFragment struct {
	ticks int
	pos   *mathutil.Vector2D
	v     *mathutil.Vector2D
}

func (f *EnemyFragment) update() error {
	f.pos = f.pos.Add(f.v)
	f.ticks++
	return nil
}

func (f *EnemyFragment) draw(dst *ebiten.Image) {
	opts := &ebiten.DrawImageOptions{}
	w, h := enemyImg.Size()
	opts.GeoM.Translate(-float64(w)/2, -float64(h)/2)
	opts.GeoM.Scale(10/float64(w), 10/float64(h))
	opts.GeoM.Rotate(float64(f.ticks) * math.Pi / 15)
	opts.GeoM.Translate(f.pos.X, f.pos.Y)
	dst.DrawImage(enemyImg, opts)
}

type GameMode int

const (
	GameModeTitle GameMode = iota
	GameModePlaying
	GameModeGameOver
)

type Game struct {
	touches                   []touchutil.Touch
	random                    *rand.Rand
	mode                      GameMode
	ticksFromModeStart        uint64
	player                    *Player
	enemy                     *Enemy
	bullets                   []*Bullet
	playerBullets             []*PlayerBullet
	flashEffects              []*FlashEffect
	enemyFragments            []*EnemyFragment
	score                     int
	graze                     int
	failuresInBulletMLRunning int
}

func (g *Game) Update() error {
	g.touches = touchutil.AppendNewTouches(g.touches)
	for _, t := range g.touches {
		t.Update()
	}

	g.ticksFromModeStart++

	switch g.mode {
	case GameModeTitle:
		if len(g.touches) > 0 && g.touches[0].IsJustTouched() {
			g.player.pos = mathutil.NewVector2D(playerHomeX, playerHomeY)

			g.setNextMode(GameModePlaying)
		}

	case GameModePlaying:
		if !g.player.invincible() {
			playerTopLeftX := math.Min(g.player.pos.X-g.player.grazeR, g.player.prevPos.X-g.player.grazeR)
			playerTopLeftY := math.Min(g.player.pos.Y-g.player.grazeR, g.player.prevPos.Y-g.player.grazeR)
			playerBottomRightX := math.Max(g.player.pos.X+g.player.grazeR, g.player.prevPos.X+g.player.grazeR)
			playerBottomRightY := math.Max(g.player.pos.Y+g.player.grazeR, g.player.prevPos.Y+g.player.grazeR)
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

				playerV := g.player.prevPos.Sub(g.player.pos)
				bulletV := b.prevPos.Sub(b.pos)
				if !b.grazed && mathutil.CapsulesCollide(
					g.player.pos, playerV, g.player.grazeR,
					b.pos, bulletV, b.r,
				) {
					g.graze++
					g.score += grazeGain

					f := &FlashEffect{
						pos:   b.pos.Add(g.player.pos).Div(2),
						v:     b.pos.Sub(g.player.pos).Normalize().Mul(0.25),
						r:     7,
						color: color.RGBA{0xff, 0, 0, 0x70},
						until: 25,
					}
					g.flashEffects = append(g.flashEffects, f)
					b.grazed = true
				}

				if mathutil.CapsulesCollide(
					g.player.pos, playerV, g.player.r,
					b.pos, bulletV, b.r,
				) {
					b.hit = true
					g.player.hit = true

					_bullets := g.bullets[:0]
					for _, b := range g.bullets {
						if b.pos.Sub(mathutil.NewVector2D(playerHomeX, playerHomeY)).NormSq() > 300*300 {
							_bullets = append(_bullets, b)
						} else {
							f := &FlashEffect{
								pos:   b.pos.Clone(),
								r:     10,
								color: color.Gray{0x70},
								until: 25,
							}
							g.flashEffects = append(g.flashEffects, f)
						}
					}
					g.bullets = _bullets

					break
				}
			}

			if mathutil.CapsulesCollide(
				g.player.pos, g.player.prevPos.Sub(g.player.pos), g.player.r,
				g.enemy.pos, g.enemy.prevPos.Sub(g.enemy.pos), g.enemy.r,
			) {
				g.player.hit = true
			}

			if g.player.hit {
				g.touches = nil

				f := &FlashEffect{
					pos:   g.player.pos.Clone(),
					r:     40,
					color: color.RGBA{0xff, 0, 0, 0xff},
					until: 25,
				}
				g.flashEffects = append(g.flashEffects, f)
			}
		}

		for _, b := range g.playerBullets {
			if mathutil.CapsulesCollide(
				g.enemy.pos, g.enemy.prevPos.Sub(g.enemy.pos), g.enemy.r,
				b.pos, b.prevPos.Sub(b.pos), b.r,
			) {
				b.hit = true
				g.enemy.hit = true

				f := &FlashEffect{
					pos:   b.pos.Clone().Add(mathutil.NewVector2D(10*g.random.Float64()-5, 10*g.random.Float64()-5)),
					r:     10,
					color: color.Gray{0x70},
					until: 25,
				}
				g.flashEffects = append(g.flashEffects, f)
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

		for _, e := range g.flashEffects {
			if err := e.update(); err != nil {
				return err
			}
		}

		for _, f := range g.enemyFragments {
			if err := f.update(); err != nil {
				return err
			}
		}

		_bullets := g.bullets[:0]
		for _, b := range g.bullets {
			if !b.hit &&
				!b.runner.Vanished() &&
				b.prevPos.X+b.r > 0 && b.prevPos.X-b.r < screenWidth && b.prevPos.Y+b.r > 0 && b.prevPos.Y-b.r < screenHeight {
				_bullets = append(_bullets, b)
			}
		}
		g.bullets = _bullets

		_playerBullets := g.playerBullets[:0]
		for _, b := range g.playerBullets {
			if !b.hit &&
				b.prevPos.X+b.r > 0 && b.prevPos.X-b.r < screenWidth && b.prevPos.Y+b.r > 0 && b.prevPos.Y-b.r < screenHeight {
				_playerBullets = append(_playerBullets, b)
			}
		}
		g.playerBullets = _playerBullets

		_flashEffects := g.flashEffects[:0]
		for _, e := range g.flashEffects {
			if !e.finished {
				_flashEffects = append(_flashEffects, e)
			}
		}
		g.flashEffects = _flashEffects

		_enemyFragments := g.enemyFragments[:0]
		for _, f := range g.enemyFragments {
			if f.pos.Sub(mathutil.NewVector2D(screenWidth/2, screenHeight/2)).NormSq() < 500*500 {
				_enemyFragments = append(_enemyFragments, f)
			}
		}
		g.enemyFragments = _enemyFragments

		if g.player.life <= 0 || g.enemy.state == EnemyStateExploded {
			g.setNextMode(GameModeGameOver)
		}

	case GameModeGameOver:
		if err := g.player.update(); err != nil {
			return err
		}

		for i, n := 0, len(g.playerBullets); i < n; i++ {
			if err := g.playerBullets[i].update(); err != nil {
				return err
			}
		}

		for _, e := range g.flashEffects {
			if err := e.update(); err != nil {
				return err
			}
		}

		for _, f := range g.enemyFragments {
			if err := f.update(); err != nil {
				return err
			}
		}

		_playerBullets := g.playerBullets[:0]
		for _, b := range g.playerBullets {
			if !b.hit &&
				b.prevPos.X+b.r > 0 && b.prevPos.X-b.r < screenWidth && b.prevPos.Y+b.r > 0 && b.prevPos.Y-b.r < screenHeight {
				_playerBullets = append(_playerBullets, b)
			}
		}
		g.playerBullets = _playerBullets

		_flashEffects := g.flashEffects[:0]
		for _, e := range g.flashEffects {
			if !e.finished {
				_flashEffects = append(_flashEffects, e)
			}
		}
		g.flashEffects = _flashEffects

		_enemyFragments := g.enemyFragments[:0]
		for _, f := range g.enemyFragments {
			if f.pos.Sub(mathutil.NewVector2D(screenWidth/2, screenHeight/2)).NormSq() < 500*500 {
				_enemyFragments = append(_enemyFragments, f)
			}
		}
		g.enemyFragments = _enemyFragments

		if g.ticksFromModeStart > 120 && len(g.touches) > 0 && g.touches[0].IsJustTouched() {
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

func (g *Game) drawTitleText(screen *ebiten.Image) {
	titleTexts := []string{"BULLET HELL"}
	for i, s := range titleTexts {
		text.Draw(screen, s, fontL.Face, screenWidth/2-len(s)*int(fontL.FaceOptions.Size)/2, 85+i*int(fontL.FaceOptions.Size*1.8), color.Black)
	}

	usageTexts := []string{"[DRAG] Move"}
	for i, s := range usageTexts {
		text.Draw(screen, s, fontS.Face, screenWidth/2-len(s)*int(fontS.FaceOptions.Size)/2, 280+i*int(fontS.FaceOptions.Size*1.8), color.Black)
	}

	creditTexts := []string{"CREATOR: NAOKI TSUJIO", "FONT: Press Start 2P by CodeMan38", "SOUND EFFECT: MaouDamashii", "POWERED BY Ebitengine"}
	for i, s := range creditTexts {
		text.Draw(screen, s, fontS.Face, screenWidth/2-len(s)*int(fontS.FaceOptions.Size)/2, 400+i*int(fontS.FaceOptions.Size*1.8), color.Black)
	}
}

func (g *Game) drawGameOverText(screen *ebiten.Image) {
	var gameOverTexts []string
	if g.enemy.state == EnemyStateExploded {
		gameOverTexts = []string{"GAME CLEAR"}
	} else {
		gameOverTexts = []string{"GAME OVER"}
	}
	for i, s := range gameOverTexts {
		text.Draw(screen, s, fontL.Face, screenWidth/2-len(s)*int(fontL.FaceOptions.Size)/2, 170+i*int(fontL.FaceOptions.Size*1.8), color.Black)
	}

	scoreText := []string{"YOUR SCORE IS", fmt.Sprintf("%s!", commaInt(g.score))}
	for i, s := range scoreText {
		text.Draw(screen, s, fontM.Face, screenWidth/2-len(s)*int(fontM.FaceOptions.Size)/2, 230+i*int(fontM.FaceOptions.Size*1.8), color.Black)
	}
}

func (g *Game) drawTopMenu(screen *ebiten.Image) {
	text.Draw(screen, fmt.Sprintf("%.1ffps", ebiten.ActualFPS()), fontSS.Face, 5, 15, color.Gray{0x70})

	for i := 0; i < len(bulletMLs)-g.enemy.bulletMLIndex; i++ {
		opts := &ebiten.DrawImageOptions{}
		w, h := enemyImg.Size()
		opts.GeoM.Translate(-float64(w)/2, -float64(h)/2)
		opts.GeoM.Scale(7/float64(w), 7/float64(h))
		opts.GeoM.Translate(float64(90+11*i), 11)
		opts.ColorScale.ScaleAlpha(0.5)
		screen.DrawImage(enemyImg, opts)
	}

	scoreText := fmt.Sprintf("SCORE %s", commaInt(g.score))
	text.Draw(screen, scoreText, fontSS.Face, screenWidth-5-len(scoreText)*8, 15, color.Gray{0x70})
}

func (g *Game) Draw(screen *ebiten.Image) {
	screen.Fill(color.White)

	switch g.mode {
	case GameModeTitle:
		g.player.draw(screen)

		g.enemy.draw(screen)

		g.drawTitleText(screen)
	case GameModePlaying:
		g.player.draw(screen)

		for _, b := range g.bullets {
			b.draw(screen)
		}

		g.enemy.draw(screen)

		for _, b := range g.playerBullets {
			b.draw(screen)
		}

		for _, e := range g.flashEffects {
			e.draw(screen)
		}

		for _, f := range g.enemyFragments {
			f.draw(screen)
		}

		g.drawTopMenu(screen)
	case GameModeGameOver:
		g.player.draw(screen)

		for _, b := range g.bullets {
			b.draw(screen)
		}

		g.enemy.draw(screen)

		for _, b := range g.playerBullets {
			b.draw(screen)
		}

		for _, e := range g.flashEffects {
			e.draw(screen)
		}

		for _, f := range g.enemyFragments {
			f.draw(screen)
		}

		g.drawGameOverText(screen)

		g.drawTopMenu(screen)
	}
}

func (g *Game) Layout(outsideWidth, outsideHeight int) (int, int) {
	return screenWidth, screenHeight
}

func (g *Game) setBulletML(index int) error {
	bml := bulletMLs[index]

	enemyRunner := true
	opts := &bulletml.NewRunnerOptions{
		OnBulletFired: func(br bulletml.BulletRunner, fc *bulletml.FireContext) {
			if enemyRunner {
				g.enemy.runner = br
				g.enemy.pos.X, g.enemy.pos.Y = br.Position()
				enemyRunner = false
			} else {
				x, y := br.Position()
				b := &Bullet{
					pos:     mathutil.NewVector2D(x, y),
					prevPos: mathutil.NewVector2D(x, y),
					r:       bulletR,
					runner:  br,
					game:    g,
				}
				g.bullets = append(g.bullets, b)
			}
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

	if err := runner.Update(); err != nil {
		return err
	}

	return nil
}

func commaInt(v int) string {
	s := []byte(strconv.Itoa(v))
	cnt := (len(s) - 1) / 3
	r := make([]byte, len(s)+cnt)
	for i := len(s) - 1; i >= 0; i-- {
		r[i+cnt] = s[i]
		if (len(s)-i)%3 == 0 && i > 0 {
			cnt--
			r[i+cnt] = byte(',')
		}
	}
	return string(r)
}

func (g *Game) setNextMode(mode GameMode) {
	g.mode = mode
	g.ticksFromModeStart = 0
}

func (g *Game) initialize() {
	g.touches = nil

	playerPos := mathutil.NewVector2D(playerHomeX, playerHomeY-45)
	g.player = &Player{
		pos:             playerPos,
		prevPos:         playerPos,
		r:               playerR,
		grazeR:          playerGrazeR,
		life:            playerInitialLife,
		invincibleUntil: -1,
		game:            g,
	}

	enemyPos := mathutil.NewVector2D(enemyHomeX, enemyHomeY+80)
	g.enemy = &Enemy{
		pos:                 enemyPos,
		prevPos:             enemyPos,
		r:                   enemyR,
		state:               EnemyStateWaiting,
		life:                100,
		startNextBulletMLAt: 180,
		game:                g,
	}

	g.bullets = nil
	g.playerBullets = nil
	g.flashEffects = nil
	g.enemyFragments = nil
	g.graze = 0
	g.failuresInBulletMLRunning = 0
	g.score = 0

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
