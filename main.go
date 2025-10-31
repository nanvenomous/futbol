package main

import (
	"fmt"
	"math/rand"
	"os"
	"time"

	"golang.org/x/term"
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

	// Dribbling randomness (0.0 = none, 1.0 = max)
	dribbleRandomness = 0

	// Player starting positions (offset from edges)
	playerStartOffset = 6
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
	running      bool
	lastBallHit  rune
	playerAStart Position
	playerBStart Position
	ballStart    Position
	keysPressed  map[byte]bool
}

func main() {
	var err = game()
	if err != nil {
		panic(err)
	}
}

func game() error {
	// rand.Seed(time.Now().UnixNano())

	playerAStart := Position{x: playerStartOffset, y: fieldHeight / 2}
	playerBStart := Position{x: fieldWidth - playerStartOffset - 1, y: fieldHeight / 2}
	ballStart := Position{x: fieldWidth / 2, y: fieldHeight / 2}

	g := &Game{
		playerA:      &Player{pos: playerAStart, char: 'a'},
		playerB:      &Player{pos: playerBStart, char: 'b'},
		ball:         &Ball{pos: ballStart},
		running:      true,
		playerAStart: playerAStart,
		playerBStart: playerBStart,
		ballStart:    ballStart,
		keysPressed:  make(map[byte]bool),
	}

	oldState, err := term.MakeRaw(int(os.Stdin.Fd()))
	if err != nil {
		return err
	}
	defer term.Restore(int(os.Stdin.Fd()), oldState)

	// Clear screen and hide cursor
	fmt.Print("\033[2J\033[?25l")

	inputChan := make(chan byte, 10)
	go readInput(inputChan)

	ticker := time.NewTicker(tickRate)
	defer ticker.Stop()

	for g.running {
		select {
		case <-ticker.C:
			g.handleMovement()
			g.updateBall()
			g.render()

			if g.playerA.score >= winScore || g.playerB.score >= winScore {
				g.running = false
			}
		case key := <-inputChan:
			g.handleInput(key)
		default:
			// Non-blocking
		}
	}

	// Show cursor again
	fmt.Print("\033[?25h\033[2J\033[H")
	fmt.Print("\r\n\r\n")
	fmt.Print("GAME OVER!\r\n")
	fmt.Printf("Player A: %d\r\n", g.playerA.score)
	fmt.Printf("Player B: %d\r\n", g.playerB.score)
	if g.playerA.score > g.playerB.score {
		fmt.Print("Player A wins!\r\n")
	} else {
		fmt.Print("Player B wins!\r\n")
	}
	fmt.Print("\r\n")

	return nil
}

func readInput(ch chan byte) {
	buf := make([]byte, 1)
	for {
		n, err := os.Stdin.Read(buf)
		if err != nil || n == 0 {
			continue
		}
		ch <- buf[0]
	}
}

func (g *Game) handleInput(key byte) {
	switch key {
	case 'q':
		g.running = false
	case 'w', 's', 'a', 'd', 'i', 'k', 'j', 'l':
		g.keysPressed[key] = true
	case 27: // ESC - in case terminal sends escape sequences
		// Clear any stuck keys
		g.keysPressed = make(map[byte]bool)
	}
}

func (g *Game) handleMovement() {
	// Player A movement
	dx, dy := 0, 0
	if g.keysPressed['w'] {
		dy = -1
	}
	if g.keysPressed['s'] {
		dy = 1
	}
	if g.keysPressed['a'] {
		dx = -1
	}
	if g.keysPressed['d'] {
		dx = 1
	}
	if dx != 0 || dy != 0 {
		g.movePlayer(g.playerA, dx, dy)
	}

	// Player B movement
	dx, dy = 0, 0
	if g.keysPressed['i'] {
		dy = -1
	}
	if g.keysPressed['k'] {
		dy = 1
	}
	if g.keysPressed['j'] {
		dx = -1
	}
	if g.keysPressed['l'] {
		dx = 1
	}
	if dx != 0 || dy != 0 {
		g.movePlayer(g.playerB, dx, dy)
	}

	// Clear key states after processing (key must be held continuously)
	g.keysPressed = make(map[byte]bool)
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
		// Check if player is moving towards the ball
		// Player should be on the opposite side of where they're moving
		movingTowardsBall := false
		if dx != 0 && dx == sign(ballDx) && ballDy == 0 {
			// Moving horizontally towards ball that's directly left/right
			movingTowardsBall = true
		} else if dy != 0 && dy == sign(ballDy) && ballDx == 0 {
			// Moving vertically towards ball that's directly up/down
			movingTowardsBall = true
		}

		if movingTowardsBall {
			// Push the ball in the direction of movement with randomness
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

func (g *Game) updateBall() {
	if g.ball.vx == 0 && g.ball.vy == 0 {
		return
	}

	g.ball.pos.x += g.ball.vx
	g.ball.pos.y += g.ball.vy

	if g.ball.pos.y <= 0 || g.ball.pos.y >= fieldHeight-1 {
		g.ball.vy = -g.ball.vy
		g.ball.pos.y += g.ball.vy
	}

	goalTop := fieldHeight/2 - goalSize/2
	goalBottom := fieldHeight/2 + goalSize/2

	if g.ball.pos.x <= 0 {
		if g.ball.pos.y >= goalTop && g.ball.pos.y <= goalBottom {
			g.playerB.score++
			g.resetBall()
		} else {
			g.ball.vx = -g.ball.vx
			g.ball.pos.x = 1
		}
	}

	if g.ball.pos.x >= fieldWidth-1 {
		if g.ball.pos.y >= goalTop && g.ball.pos.y <= goalBottom {
			g.playerA.score++
			g.resetBall()
		} else {
			g.ball.vx = -g.ball.vx
			g.ball.pos.x = fieldWidth - 2
		}
	}

	// Apply friction
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

	if g.ball.pos.x == g.playerA.pos.x && g.ball.pos.y == g.playerA.pos.y {
		g.ball.vx = 2
		g.lastBallHit = g.playerA.char
	}
	if g.ball.pos.x == g.playerB.pos.x && g.ball.pos.y == g.playerB.pos.y {
		g.ball.vx = -2
		g.lastBallHit = g.playerB.char
	}
}

func (g *Game) resetBall() {
	g.ball.pos = g.ballStart
	g.ball.vx = 0
	g.ball.vy = 0
	g.playerA.pos = g.playerAStart
	g.playerB.pos = g.playerBStart
	time.Sleep(resetDelay)
}

func (g *Game) render() {
	// Move cursor to top-left
	fmt.Print("\033[H")

	field := make([][]rune, fieldHeight)
	for i := range field {
		field[i] = make([]rune, fieldWidth)
		for j := range field[i] {
			field[i][j] = ' '
		}
	}

	// Draw borders
	for i := 0; i < fieldWidth; i++ {
		field[0][i] = '-'
		field[fieldHeight-1][i] = '-'
	}
	for i := 0; i < fieldHeight; i++ {
		field[i][0] = '|'
		field[i][fieldWidth-1] = '|'
	}

	// Draw goals (openings in the walls)
	goalTop := fieldHeight/2 - goalSize/2
	goalBottom := fieldHeight/2 + goalSize/2
	for i := goalTop; i <= goalBottom; i++ {
		field[i][0] = ' '
		field[i][fieldWidth-1] = ' '
	}

	// Place players and ball
	field[g.playerA.pos.y][g.playerA.pos.x] = g.playerA.char
	field[g.playerB.pos.y][g.playerB.pos.x] = g.playerB.char
	field[g.ball.pos.y][g.ball.pos.x] = 'o'

	// Print score
	fmt.Printf("Player A: %d | Player B: %d | First to %d wins! | Press 'q' to quit\r\n",
		g.playerA.score, g.playerB.score, winScore)

	// Print field
	for i := range field {
		fmt.Print(string(field[i]))
		fmt.Print("\r\n")
	}

	// Print controls
	fmt.Print("Controls: Player A (WASD) | Player B (IJKL)\r\n")
}
