package main

import (
	"fmt"
	"image/color"
	"math/rand"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/text"
	"github.com/hajimehoshi/ebiten/v2/vector"
	"golang.org/x/image/font/basicfont"
)

// Game tuning constants
const (
	// Field dimensions
	fieldWidth  = 50
	fieldHeight = 15
	goalSize    = 5 // Height of goal opening (total will be goalSize, centered)

	// Gameplay
	winScore   = 5
	tickRate   = 50 * time.Millisecond
	resetDelay = 1 * time.Second

	// Ball physics
	dribblePowerHorizontal = 2 // Horizontal kick strength
	dribblePowerVertical   = 1 // Vertical kick strength
	ballFriction           = 1 // Amount velocity decreases per tick
	maxBallSpeed           = 3

	// Dribbling randomness (0.0 = none, 1.0 = max)
	dribbleRandomness = 0

	// Player starting positions (offset from edges)
	playerStartOffset = 6

	// Rendering
	cellSize     = 16
	topMargin    = 48
	bottomMargin = 48
	screenWidth  = fieldWidth * cellSize
	screenHeight = topMargin + fieldHeight*cellSize + bottomMargin
	borderWidth  = 2.0
)

var (
	backgroundColor = color.RGBA{R: 12, G: 28, B: 44, A: 255}
	pitchColor      = color.RGBA{R: 22, G: 90, B: 52, A: 255}
	borderColor     = color.RGBA{R: 220, G: 220, B: 220, A: 255}
	playerAColor    = color.RGBA{R: 244, G: 105, B: 98, A: 255}
	playerBColor    = color.RGBA{R: 93, G: 173, B: 255, A: 255}
	ballColor       = color.RGBA{R: 255, G: 235, B: 153, A: 255}
)

type Position struct {
	x, y int
}

type Player struct {
	pos   Position
	char  rune
	score int
}

type Ball struct {
	pos Position
	vx  int
	vy  int
}

type Game struct {
	playerA      *Player
	playerB      *Player
	ball         *Ball
	lastBallHit  rune
	playerAStart Position
	playerBStart Position
	ballStart    Position

	statusText  string
	lastTick    time.Time
	accumulator time.Duration
	resetTimer  time.Duration
	gameOver    bool
}

func main() {
	rand.Seed(time.Now().UnixNano())

	game := newGame()

	ebiten.SetWindowTitle("Footerm")
	ebiten.SetWindowResizingMode(ebiten.WindowResizingModeEnabled)
	ebiten.SetWindowSize(screenWidth, screenHeight)

	if err := ebiten.RunGame(game); err != nil && err != ebiten.Termination {
		panic(err)
	}
}

func newGame() *Game {
	playerAStart := Position{x: playerStartOffset, y: fieldHeight / 2}
	playerBStart := Position{x: fieldWidth - playerStartOffset - 1, y: fieldHeight / 2}
	ballStart := Position{x: fieldWidth / 2, y: fieldHeight / 2}

	return &Game{
		playerA:      &Player{pos: playerAStart, char: 'A'},
		playerB:      &Player{pos: playerBStart, char: 'B'},
		ball:         &Ball{pos: ballStart},
		playerAStart: playerAStart,
		playerBStart: playerBStart,
		ballStart:    ballStart,
		statusText:   "Kick-off!",
		lastTick:     time.Now(),
	}
}

func (g *Game) Update() error {
	if ebiten.IsKeyPressed(ebiten.KeyQ) {
		return ebiten.Termination
	}

	now := time.Now()
	elapsed := now.Sub(g.lastTick)
	g.lastTick = now

	if g.gameOver {
		if ebiten.IsKeyPressed(ebiten.KeySpace) {
			g.resetMatch()
		}
		return nil
	}

	if g.resetTimer > 0 {
		if elapsed >= g.resetTimer {
			g.resetTimer = 0
			g.statusText = "Kick-off!"
		} else {
			g.resetTimer -= elapsed
		}
		return nil
	}

	g.accumulator += elapsed
	for g.accumulator >= tickRate {
		g.accumulator -= tickRate

		g.handleMovement()
		g.updateBall()

		if g.playerA.score >= winScore || g.playerB.score >= winScore {
			if g.playerA.score > g.playerB.score {
				g.statusText = "Player A wins! Press Space to restart."
			} else {
				g.statusText = "Player B wins! Press Space to restart."
			}
			g.gameOver = true
			break
		}
	}

	return nil
}

func (g *Game) Draw(screen *ebiten.Image) {
	screen.Fill(backgroundColor)

	// Draw pitch background
	fillRect(screen, 0, float64(topMargin), float64(screenWidth), float64(fieldHeight*cellSize), pitchColor)

	drawBorders(screen)

	// Draw players and ball
	drawPlayer(screen, g.playerA.pos, playerAColor)
	drawPlayer(screen, g.playerB.pos, playerBColor)
	drawBall(screen, g.ball.pos)

	// Scoreboard and messages
	scoreText := fmt.Sprintf("Player A: %d   Player B: %d   First to %d", g.playerA.score, g.playerB.score, winScore)
	text.Draw(screen, scoreText, basicfont.Face7x13, 16, 24, color.White)

	status := g.statusText
	if g.resetTimer > 0 {
		countdown := fmt.Sprintf("Resuming in %.1fs", g.resetTimer.Seconds())
		if status != "" {
			status = fmt.Sprintf("%s  %s", status, countdown)
		} else {
			status = countdown
		}
	}
	if status != "" {
		text.Draw(screen, status, basicfont.Face7x13, 16, topMargin-16, color.White)
	}

	text.Draw(screen, "Controls: Player A (WASD) | Player B (IJKL) | Press Q to quit", basicfont.Face7x13,
		16, screenHeight-16, color.White)
}

func (g *Game) Layout(_, _ int) (int, int) {
	return screenWidth, screenHeight
}

func (g *Game) handleMovement() {
	// Player A movement
	dx, dy := movementVector(ebiten.KeyA, ebiten.KeyD, ebiten.KeyW, ebiten.KeyS)
	if dx != 0 || dy != 0 {
		g.movePlayer(g.playerA, dx, dy)
	}

	// Player B movement
	dx, dy = movementVector(ebiten.KeyJ, ebiten.KeyL, ebiten.KeyI, ebiten.KeyK)
	if dx != 0 || dy != 0 {
		g.movePlayer(g.playerB, dx, dy)
	}
}

func movementVector(left, right, up, down ebiten.Key) (int, int) {
	dx, dy := 0, 0

	leftPressed := ebiten.IsKeyPressed(left)
	rightPressed := ebiten.IsKeyPressed(right)
	if leftPressed != rightPressed { // only move if exactly one direction held
		if leftPressed {
			dx = -1
		} else {
			dx = 1
		}
	}

	upPressed := ebiten.IsKeyPressed(up)
	downPressed := ebiten.IsKeyPressed(down)
	if upPressed != downPressed {
		if upPressed {
			dy = -1
		} else {
			dy = 1
		}
	}

	return dx, dy
}

func (g *Game) movePlayer(p *Player, dx, dy int) {
	newX := p.pos.x + dx
	newY := p.pos.y + dy

	// Check bounds
	if newX < 1 || newX >= fieldWidth-1 || newY < 1 || newY >= fieldHeight-1 {
		return
	}

	// Check collision with other player
	otherPlayer := g.playerB
	if p == g.playerB {
		otherPlayer = g.playerA
	}
	if newX == otherPlayer.pos.x && newY == otherPlayer.pos.y {
		return // Can't move into another player
	}

	// Check if player is next to ball before moving
	ballDx := g.ball.pos.x - p.pos.x
	ballDy := g.ball.pos.y - p.pos.y
	distToBall := abs(ballDx) + abs(ballDy)

	p.pos.x = newX
	p.pos.y = newY

	// Dribbling: only when moving towards the ball from the right direction
	if distToBall == 1 {
		movingTowardsBall := false
		if dx != 0 && dx == sign(ballDx) && ballDy == 0 {
			movingTowardsBall = true
		} else if dy != 0 && dy == sign(ballDy) && ballDx == 0 {
			movingTowardsBall = true
		}

		if movingTowardsBall {
			randomX := 0.0
			randomY := 0.0
			if dribbleRandomness > 0 {
				randomX = (rand.Float64() - 0.5) * 2 * dribbleRandomness
				randomY = (rand.Float64() - 0.5) * 2 * dribbleRandomness
			}

			ballVx := int(float64(dx*dribblePowerHorizontal) * (1 + randomX))
			ballVy := int(float64(dy*dribblePowerVertical) * (1 + randomY))

			g.ball.vx = ballVx
			g.ball.vy = ballVy
			g.lastBallHit = p.char
		}
	}
}

func (g *Game) updateBall() {
	if g.ball.vx == 0 && g.ball.vy == 0 {
		return
	}

	g.ball.pos.x += g.ball.vx
	g.ball.pos.y += g.ball.vy

	bouncedX := false
	bouncedY := false

	if g.ball.pos.y <= 0 {
		g.ball.pos.y = 1
		g.bounceVertically()
		bouncedY = true
	} else if g.ball.pos.y >= fieldHeight-1 {
		g.ball.pos.y = fieldHeight - 2
		g.bounceVertically()
		bouncedY = true
	}

	goalTop := fieldHeight/2 - goalSize/2
	goalBottom := fieldHeight/2 + goalSize/2

	if g.ball.pos.x <= 0 {
		if g.ball.pos.y >= goalTop && g.ball.pos.y <= goalBottom {
			g.playerB.score++
			g.resetAfterGoal("Player B scores!")
		} else {
			g.ball.pos.x = 1
			g.bounceHorizontally()
			bouncedX = true
		}
	} else if g.ball.pos.x >= fieldWidth-1 {
		if g.ball.pos.y >= goalTop && g.ball.pos.y <= goalBottom {
			g.playerA.score++
			g.resetAfterGoal("Player A scores!")
		} else {
			g.ball.pos.x = fieldWidth - 2
			g.bounceHorizontally()
			bouncedX = true
		}
	}

	// Apply friction
	if !bouncedX {
		if g.ball.vx > 0 {
			g.ball.vx -= ballFriction
			if g.ball.vx < 0 {
				g.ball.vx = 0
			}
		} else if g.ball.vx < 0 {
			g.ball.vx += ballFriction
			if g.ball.vx > 0 {
				g.ball.vx = 0
			}
		}
	}

	if !bouncedY {
		if g.ball.vy > 0 {
			g.ball.vy -= ballFriction
			if g.ball.vy < 0 {
				g.ball.vy = 0
			}
		} else if g.ball.vy < 0 {
			g.ball.vy += ballFriction
			if g.ball.vy > 0 {
				g.ball.vy = 0
			}
		}
	}

	if g.ball.pos.x == g.playerA.pos.x && g.ball.pos.y == g.playerA.pos.y {
		g.ball.vx = 2
		g.lastBallHit = g.playerA.char
	}
	if g.ball.pos.x == g.playerB.pos.x && g.ball.pos.y == g.playerB.pos.y {
		g.ball.vx = -2
		g.lastBallHit = g.playerB.char
	}
}

func (g *Game) bounceHorizontally() {
	g.ball.vx = clampNonZero(-g.ball.vx)
	g.ball.vy = randomizeBounceSpeed(g.ball.vy)
}

func (g *Game) bounceVertically() {
	g.ball.vy = clampNonZero(-g.ball.vy)
	g.ball.vx = randomizeBounceSpeed(g.ball.vx)
}

func (g *Game) resetAfterGoal(message string) {
	g.ball.pos = g.ballStart
	g.ball.vx = 0
	g.ball.vy = 0
	g.playerA.pos = g.playerAStart
	g.playerB.pos = g.playerBStart
	g.accumulator = 0
	g.resetTimer = resetDelay
	g.statusText = message
}

func (g *Game) resetMatch() {
	g.playerA.score = 0
	g.playerB.score = 0
	g.ball.pos = g.ballStart
	g.ball.vx = 0
	g.ball.vy = 0
	g.playerA.pos = g.playerAStart
	g.playerB.pos = g.playerBStart
	g.accumulator = 0
	g.resetTimer = 0
	g.statusText = "Kick-off!"
	g.gameOver = false
}

func drawBorders(screen *ebiten.Image) {
	goalTop := fieldHeight/2 - goalSize/2
	goalBottom := fieldHeight/2 + goalSize/2

	// Top and bottom borders
	fillRect(screen, 0, float64(topMargin), float64(screenWidth), borderWidth, borderColor)
	fillRect(screen, 0, float64(topMargin+fieldHeight*cellSize)-borderWidth, float64(screenWidth), borderWidth, borderColor)

	// Left border (split for goal)
	if goalTop > 0 {
		height := float64(goalTop * cellSize)
		fillRect(screen, 0, float64(topMargin), borderWidth, height, borderColor)
	}
	if goalBottom < fieldHeight-1 {
		startY := float64(topMargin + (goalBottom+1)*cellSize)
		height := float64((fieldHeight - goalBottom - 1) * cellSize)
		if height > 0 {
			fillRect(screen, 0, startY, borderWidth, height, borderColor)
		}
	}

	// Right border (split for goal)
	x := float64(screenWidth) - borderWidth
	if goalTop > 0 {
		height := float64(goalTop * cellSize)
		fillRect(screen, x, float64(topMargin), borderWidth, height, borderColor)
	}
	if goalBottom < fieldHeight-1 {
		startY := float64(topMargin + (goalBottom+1)*cellSize)
		height := float64((fieldHeight - goalBottom - 1) * cellSize)
		if height > 0 {
			fillRect(screen, x, startY, borderWidth, height, borderColor)
		}
	}
}

func drawPlayer(screen *ebiten.Image, pos Position, col color.Color) {
	margin := float64(cellSize) * 0.2
	x := float64(pos.x*cellSize) + margin
	y := float64(topMargin+pos.y*cellSize) + margin
	size := float64(cellSize) - margin*2
	fillRect(screen, x, y, size, size, col)
}

func drawBall(screen *ebiten.Image, pos Position) {
	margin := float64(cellSize) * 0.35
	x := float64(pos.x*cellSize) + margin
	y := float64(topMargin+pos.y*cellSize) + margin
	size := float64(cellSize) - margin*2
	fillRect(screen, x, y, size, size, ballColor)
}

func fillRect(dst *ebiten.Image, x, y, width, height float64, clr color.Color) {
	vector.DrawFilledRect(dst, float32(x), float32(y), float32(width), float32(height), clr, false)
}

func clampNonZero(v int) int {
	if v > 0 {
		if v > maxBallSpeed {
			return maxBallSpeed
		}
		return v
	}
	if v < 0 {
		if v < -maxBallSpeed {
			return -maxBallSpeed
		}
		return v
	}
	return randomSign()
}

func randomizeBounceSpeed(current int) int {
	updated := current + rand.Intn(3) - 1
	if updated > maxBallSpeed {
		updated = maxBallSpeed
	} else if updated < -maxBallSpeed {
		updated = -maxBallSpeed
	}
	if updated == 0 {
		updated = randomSign()
	}
	return updated
}

func randomSign() int {
	if rand.Intn(2) == 0 {
		return -1
	}
	return 1
}

func sign(x int) int {
	if x > 0 {
		return 1
	} else if x < 0 {
		return -1
	}
	return 0
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}
