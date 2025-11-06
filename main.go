package main

import (
	"fmt"
	"image/color"
	"math"
	"math/rand"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	text "github.com/hajimehoshi/ebiten/v2/text/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
	"golang.org/x/image/font/basicfont"
)

// Pitch dimensions expressed in logical grid units.
const (
	fieldWidth  = 50
	fieldHeight = 25
	goalSize    = 8

	winScore   = 5
	tickRate   = 50 * time.Millisecond
	resetDelay = 1 * time.Second
)

// Player tuning.
const (
	playerRadius        = 0.65
	playerMaxSpeed      = 0.85
	playerAcceleration  = 0.22
	playerDrag          = 0.82
	playerBumpStiffness = 0.35
)

// Ball tuning.
const (
	ballRadius        = 0.35
	ballDrag          = 0.85
	ballRestThreshold = 0.02
	ballBounceDamp    = 0.88
	kickImpulse       = 1.25
	maxBallSpeed      = 1.6
)

// Rendering constants.
const (
	cellSize     = 18
	topMargin    = 64
	bottomMargin = 64
	screenWidth  = fieldWidth * cellSize
	screenHeight = topMargin + fieldHeight*cellSize + bottomMargin
)

var (
	backgroundColor = color.RGBA{R: 12, G: 28, B: 44, A: 255}
	pitchColor      = color.RGBA{R: 24, G: 110, B: 62, A: 255}
	lineColor       = color.RGBA{R: 220, G: 230, B: 220, A: 255}
	playerAColor    = color.RGBA{R: 255, G: 121, B: 97, A: 255}
	playerBColor    = color.RGBA{R: 114, G: 184, B: 255, A: 255}
	playerAShadow   = color.RGBA{R: 145, G: 60, B: 48, A: 120}
	playerBShadow   = color.RGBA{R: 62, G: 110, B: 162, A: 120}
	ballColor       = color.RGBA{R: 250, G: 240, B: 200, A: 255}
	shadowColor     = color.RGBA{R: 0, G: 0, B: 0, A: 90}
)
var (
	gameStart = time.Now()
	hudFace   = text.NewGoXFace(basicfont.Face7x13)
	hudAscent = hudFace.Metrics().HAscent
)

type vec2 struct {
	X, Y float64
}

func (v vec2) Add(o vec2) vec2 { return vec2{v.X + o.X, v.Y + o.Y} }
func (v vec2) Sub(o vec2) vec2 { return vec2{v.X - o.X, v.Y - o.Y} }
func (v vec2) Scale(s float64) vec2 {
	return vec2{v.X * s, v.Y * s}
}
func (v vec2) Length() float64 {
	return math.Hypot(v.X, v.Y)
}
func (v vec2) Normalize() vec2 {
	if v.X == 0 && v.Y == 0 {
		return vec2{}
	}
	l := v.Length()
	if l == 0 {
		return vec2{}
	}
	return vec2{v.X / l, v.Y / l}
}
func (v vec2) Dot(o vec2) float64 {
	return v.X*o.X + v.Y*o.Y
}

func approachVelocity(current, target vec2, accel float64) vec2 {
	delta := target.Sub(current)
	dist := delta.Length()
	if dist <= accel {
		return target
	}
	return current.Add(delta.Scale(accel / dist))
}

func limit(v vec2, max float64) vec2 {
	len := v.Length()
	if len == 0 || len <= max {
		return v
	}
	return v.Scale(max / len)
}

type Player struct {
	pos     vec2
	vel     vec2
	score   int
	color   color.RGBA
	shadow  color.RGBA
	lastDir vec2
	name    string
}

type Ball struct {
	pos vec2
	vel vec2
}

type Game struct {
	playerA *Player
	playerB *Player
	ball    *Ball

	lastTick    time.Time
	accumulator time.Duration

	resetTimer time.Duration
	status     string
	gameOver   bool
}

func main() {
	// deprecated: rand.Seed is deprecated: As of Go 1.20 there is no reason to call Seed with a random value.
	// rand.Seed(time.Now().UnixNano())

	g := newGame()

	ebiten.SetWindowTitle("Footerm")
	ebiten.SetWindowResizingMode(ebiten.WindowResizingModeEnabled)
	ebiten.SetWindowSize(screenWidth, screenHeight)

	if err := ebiten.RunGame(g); err != nil && err != ebiten.Termination {
		panic(err)
	}
}

func newGame() *Game {
	playerAStart := vec2{X: 6, Y: fieldHeight/2 - 2}
	playerBStart := vec2{X: float64(fieldWidth - 6), Y: fieldHeight/2 + 2}
	ballStart := vec2{X: fieldWidth / 2, Y: fieldHeight / 2}

	return &Game{
		playerA:  &Player{pos: playerAStart, color: playerAColor, shadow: playerAShadow, lastDir: vec2{X: 1}, name: "Player A"},
		playerB:  &Player{pos: playerBStart, color: playerBColor, shadow: playerBShadow, lastDir: vec2{X: -1}, name: "Player B"},
		ball:     &Ball{pos: ballStart},
		status:   "Kick-off!",
		lastTick: time.Now(),
	}
}

func (g *Game) Update() error {
	now := time.Now()
	dt := now.Sub(g.lastTick)
	g.lastTick = now

	if g.gameOver {
		if ebiten.IsKeyPressed(ebiten.KeySpace) {
			g.resetMatch()
		}
		return nil
	}

	if g.resetTimer > 0 {
		if dt >= g.resetTimer {
			g.resetTimer = 0
			g.status = "Kick-off!"
		} else {
			g.resetTimer -= dt
		}
		return nil
	}

	g.accumulator += dt
	for g.accumulator >= tickRate {
		g.stepSimulation()
		g.accumulator -= tickRate
	}

	return nil
}

func (g *Game) stepSimulation() {
	g.handleMovement()
	g.resolvePlayerCollision()
	g.updateBall()

	if g.playerA.score >= winScore || g.playerB.score >= winScore {
		if g.playerA.score > g.playerB.score {
			g.status = "Player A wins! Press Space to restart."
		} else {
			g.status = "Player B wins! Press Space to restart."
		}
		g.gameOver = true
	}
}

func (g *Game) handleMovement() {
	g.updatePlayer(g.playerA, movementInput(
		ebiten.KeyA, ebiten.KeyD, ebiten.KeyW, ebiten.KeyS))
	g.updatePlayer(g.playerB, movementInput(
		ebiten.KeyJ, ebiten.KeyL, ebiten.KeyI, ebiten.KeyK))
}

func (g *Game) updatePlayer(p *Player, input vec2) {
	if input.Length() > 0 {
		p.lastDir = input
		target := input.Scale(playerMaxSpeed)
		p.vel = approachVelocity(p.vel, target, playerAcceleration)
	} else {
		p.vel = p.vel.Scale(playerDrag)
		if p.vel.Length() < 0.01 {
			p.vel = vec2{}
		}
	}

	p.pos = p.pos.Add(p.vel)
	keepInsideField(p, playerRadius)
}

func movementInput(left, right, up, down ebiten.Key) vec2 {
	dir := vec2{}
	if ebiten.IsKeyPressed(left) {
		dir.X -= 1
	}
	if ebiten.IsKeyPressed(right) {
		dir.X += 1
	}
	if ebiten.IsKeyPressed(up) {
		dir.Y -= 1
	}
	if ebiten.IsKeyPressed(down) {
		dir.Y += 1
	}
	if dir.X == 0 && dir.Y == 0 {
		return dir
	}
	return dir.Normalize()
}

func keepInsideField(p *Player, radius float64) {
	if p.pos.X < radius {
		p.pos.X = radius
		p.vel.X = 0
	}
	if p.pos.X > float64(fieldWidth)-radius {
		p.pos.X = float64(fieldWidth) - radius
		p.vel.X = 0
	}
	if p.pos.Y < radius {
		p.pos.Y = radius
		p.vel.Y = 0
	}
	if p.pos.Y > float64(fieldHeight)-radius {
		p.pos.Y = float64(fieldHeight) - radius
		p.vel.Y = 0
	}
}

func (g *Game) resolvePlayerCollision() {
	delta := g.playerB.pos.Sub(g.playerA.pos)
	dist := delta.Length()
	minDist := playerRadius * 2
	if dist == 0 || dist >= minDist {
		return
	}

	normal := delta.Normalize()
	penetration := minDist - dist
	correction := normal.Scale(penetration / 2)

	g.playerA.pos = g.playerA.pos.Sub(correction)
	g.playerB.pos = g.playerB.pos.Add(correction)

	// Soften the bump by nudging velocities apart.
	g.playerA.vel = g.playerA.vel.Sub(normal.Scale(playerBumpStiffness))
	g.playerB.vel = g.playerB.vel.Add(normal.Scale(playerBumpStiffness))

	keepInsideField(g.playerA, playerRadius)
	keepInsideField(g.playerB, playerRadius)
}

func (g *Game) updateBall() {
	g.ball.pos = g.ball.pos.Add(g.ball.vel)

	bouncedX := false
	bouncedY := false

	if g.ball.pos.Y-ballRadius <= 0 {
		g.ball.pos.Y = ballRadius
		g.bounceVertically()
		bouncedY = true
	} else if g.ball.pos.Y+ballRadius >= float64(fieldHeight) {
		g.ball.pos.Y = float64(fieldHeight) - ballRadius
		g.bounceVertically()
		bouncedY = true
	}

	goalTop := float64(fieldHeight)/2 - float64(goalSize)/2
	goalBottom := goalTop + float64(goalSize)

	if g.ball.pos.X-ballRadius <= 0 {
		if g.ball.pos.Y >= goalTop && g.ball.pos.Y <= goalBottom {
			g.playerB.score++
			g.resetAfterGoal("Player B scores!")
			return
		}
		g.ball.pos.X = ballRadius
		g.bounceHorizontally()
		bouncedX = true
	} else if g.ball.pos.X+ballRadius >= float64(fieldWidth) {
		if g.ball.pos.Y >= goalTop && g.ball.pos.Y <= goalBottom {
			g.playerA.score++
			g.resetAfterGoal("Player A scores!")
			return
		}
		g.ball.pos.X = float64(fieldWidth) - ballRadius
		g.bounceHorizontally()
		bouncedX = true
	}

	g.handleBallPlayerCollision(g.playerA)
	g.handleBallPlayerCollision(g.playerB)

	if !bouncedX {
		g.ball.vel.X *= ballDrag
	}
	if !bouncedY {
		g.ball.vel.Y *= ballDrag
	}

	if g.ball.vel.Length() < ballRestThreshold {
		g.ball.vel = vec2{}
	}
}

func (g *Game) handleBallPlayerCollision(p *Player) {
	offset := g.ball.pos.Sub(p.pos)
	dist := offset.Length()
	minDist := ballRadius + playerRadius
	if dist == 0 || dist > minDist {
		return
	}

	normal := offset.Normalize()
	penetration := minDist - dist

	// Separate the ball slightly so it doesn't remain embedded.
	g.ball.pos = g.ball.pos.Add(normal.Scale(penetration + 0.02))

	relativeVel := g.ball.vel.Sub(p.vel)
	velAlongNormal := relativeVel.Dot(normal)

	if velAlongNormal < 0 {
		g.ball.vel = g.ball.vel.Sub(normal.Scale((1 + ballBounceDamp) * velAlongNormal))
	}

	kick := normal.Scale(kickImpulse)
	g.ball.vel = g.ball.vel.Add(kick).Add(p.vel.Scale(0.35))
	g.ball.vel = limit(g.ball.vel, maxBallSpeed)
}

func (g *Game) bounceHorizontally() {
	g.ball.vel.X = -g.ball.vel.X * ballBounceDamp
	g.ball.vel.Y += rand.Float64()*0.6 - 0.3
	g.ball.vel = limit(g.ball.vel, maxBallSpeed)
}

func (g *Game) bounceVertically() {
	g.ball.vel.Y = -g.ball.vel.Y * ballBounceDamp
	g.ball.vel.X += rand.Float64()*0.6 - 0.3
	g.ball.vel = limit(g.ball.vel, maxBallSpeed)
}

func (g *Game) resetAfterGoal(message string) {
	g.ball.pos = vec2{X: fieldWidth / 2, Y: fieldHeight / 2}
	g.ball.vel = vec2{}
	g.playerA.pos = vec2{X: 6, Y: fieldHeight/2 - 2}
	g.playerB.pos = vec2{X: float64(fieldWidth - 6), Y: fieldHeight/2 + 2}
	g.playerA.vel = vec2{}
	g.playerB.vel = vec2{}
	g.accumulator = 0
	g.resetTimer = resetDelay
	g.status = message
}

func (g *Game) resetMatch() {
	g.playerA.score = 0
	g.playerB.score = 0
	g.gameOver = false
	g.resetAfterGoal("Kick-off!")
	g.resetTimer = 0
	g.status = "Kick-off!"
}

func (g *Game) Draw(screen *ebiten.Image) {
	screen.Fill(backgroundColor)

	drawPitch(screen)
	drawMidfield(screen)

	drawShadow(screen, g.ball.pos, ballRadius*cellSize*0.6, shadowColor)
	drawShadow(screen, g.playerA.pos, playerRadius*cellSize, playerAShadow)
	drawShadow(screen, g.playerB.pos, playerRadius*cellSize, playerBShadow)

	drawPlayer(screen, g.playerA)
	drawPlayer(screen, g.playerB)
	drawBall(screen, g.ball)

	drawHUD(screen, g.playerA, g.playerB, g.status, g.resetTimer)
}

func drawPitch(screen *ebiten.Image) {
	fillRect(screen, 0, float64(topMargin), float64(screenWidth), float64(fieldHeight*cellSize), pitchColor)

	// Goals.
	goalTop := topMargin + (fieldHeight-goalSize)/2*cellSize
	fillRect(screen, 0, float64(goalTop), float64(cellSize/3), float64(goalSize*cellSize), lineColor)
	fillRect(screen, float64(screenWidth)-float64(cellSize/3), float64(goalTop), float64(cellSize/3), float64(goalSize*cellSize), lineColor)

	// Outer lines.
	strokeRect(screen, float64(cellSize), float64(topMargin), float64(screenWidth-2*cellSize), float64(fieldHeight*cellSize), lineColor, 2)
}

func drawMidfield(screen *ebiten.Image) {
	midX := float64(screenWidth) / 2
	fillRect(screen, midX-1, float64(topMargin), 2, float64(fieldHeight*cellSize), lineColor)

	centerY := float64(topMargin + fieldHeight*cellSize/2)
	vector.FillCircle(screen, float32(midX), float32(centerY), float32(cellSize*2), lineColor, true)
	vector.FillCircle(screen, float32(midX), float32(centerY), float32(cellSize*2-4), pitchColor, true)
}

func drawShadow(screen *ebiten.Image, pos vec2, radius float64, clr color.RGBA) {
	x, y := worldToScreen(pos)
	y += 6
	vector.FillCircle(screen, float32(x), float32(y), float32(radius), clr, true)
}

func drawPlayer(screen *ebiten.Image, p *Player) {
	x, y := worldToScreen(p.pos)
	bodyRadius := float32(playerRadius * float64(cellSize))

	vector.FillCircle(screen, float32(x), float32(y), bodyRadius, p.color, true)

	dir := p.lastDir
	if dir.Length() == 0 {
		dir = vec2{X: 1}
	}
	dir = dir.Normalize()

	eyeOffset := dir.Scale(playerRadius * float64(cellSize) * 0.45)
	eyePosX := float32(x + eyeOffset.X)
	eyePosY := float32(y + eyeOffset.Y)
	vector.FillCircle(screen, eyePosX, eyePosY, bodyRadius*0.2, color.RGBA{A: 255}, true)
}

func drawBall(screen *ebiten.Image, ball *Ball) {
	x, y := worldToScreen(ball.pos)
	r := float32(ballRadius * float64(cellSize))
	vector.FillCircle(screen, float32(x), float32(y), r, ballColor, true)

	spin := float32(math.Mod(time.Since(gameStart).Seconds()*4, 2*math.Pi))
	vector.FillCircle(screen, float32(x)+float32(math.Cos(float64(spin)))*r*0.4, float32(y)+float32(math.Sin(float64(spin)))*r*0.4, r*0.25, color.RGBA{R: 230, G: 210, B: 180, A: 255}, true)
}

func drawHUD(screen *ebiten.Image, a, b *Player, status string, resetTimer time.Duration) {
	score := fmt.Sprintf("%s: %d   %s: %d   First to %d", a.name, a.score, b.name, b.score, winScore)
	drawHUDText(screen, score, 24, 28, color.White)

	if resetTimer > 0 {
		status = fmt.Sprintf("%s  Resuming in %.1fs", status, resetTimer.Seconds())
	}
	if status != "" {
		drawHUDText(screen, status, 24, 52, color.White)
	}

	drawHUDText(screen, "Controls: Player A (WASD) | Player B (IJKL)", 24, float64(screenHeight-28), color.White)
}

func fillRect(dst *ebiten.Image, x, y, width, height float64, clr color.Color) {
	vector.FillRect(dst, float32(x), float32(y), float32(width), float32(height), clr, true)
}

func strokeRect(dst *ebiten.Image, x, y, width, height float64, clr color.Color, thickness float64) {
	t := float32(thickness)
	vector.FillRect(dst, float32(x), float32(y), float32(width), t, clr, true)
	vector.FillRect(dst, float32(x), float32(y+height-thickness), float32(width), t, clr, true)
	vector.FillRect(dst, float32(x), float32(y), t, float32(height), clr, true)
	vector.FillRect(dst, float32(x+width-thickness), float32(y), t, float32(height), clr, true)
}

func worldToScreen(pos vec2) (float64, float64) {
	return pos.X * float64(cellSize), topMargin + pos.Y*float64(cellSize)
}

func (g *Game) Layout(_, _ int) (int, int) {
	return screenWidth, screenHeight
}

func drawHUDText(screen *ebiten.Image, msg string, x, y float64, clr color.Color) {
	if msg == "" {
		return
	}

	op := &text.DrawOptions{}
	op.GeoM.Translate(x, y-hudAscent)
	if clr != nil {
		op.ColorScale.ScaleWithColor(clr)
	}
	text.Draw(screen, msg, hudFace, op)
}
