package main

import (
	"fmt"
	"os"
	"time"

	"golang.org/x/term"
)

const (
	winScore    = 5
	fieldWidth  = 50
	fieldHeight = 15
)

type Position struct {
	x, y int
}

type Player struct {
	pos    Position
	char   rune
	score  int
}

type Ball struct {
	pos Position
	vx  int
	vy  int
}

type Game struct {
	playerA     *Player
	playerB     *Player
	ball        *Ball
	running     bool
	lastBallHit rune
}

func main() {
	var err = game()
	if err != nil {
		panic(err)
	}
}

func game() error {
	g := &Game{
		playerA: &Player{pos: Position{x: 5, y: fieldHeight / 2}, char: 'a'},
		playerB: &Player{pos: Position{x: fieldWidth - 6, y: fieldHeight / 2}, char: 'b'},
		ball:    &Ball{pos: Position{x: fieldWidth / 2, y: fieldHeight / 2}},
		running: true,
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

	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()

	for g.running {
		select {
		case <-ticker.C:
			g.updateBall()
			g.render()

			if g.playerA.score >= winScore || g.playerB.score >= winScore {
				g.running = false
			}
		case key := <-inputChan:
			g.handleInput(key)
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
	case 'w':
		g.movePlayer(g.playerA, 0, -1)
	case 's':
		g.movePlayer(g.playerA, 0, 1)
	case 'a':
		g.movePlayer(g.playerA, -1, 0)
	case 'd':
		g.movePlayer(g.playerA, 1, 0)
	case 'i':
		g.movePlayer(g.playerB, 0, -1)
	case 'k':
		g.movePlayer(g.playerB, 0, 1)
	case 'j':
		g.movePlayer(g.playerB, -1, 0)
	case 'l':
		g.movePlayer(g.playerB, 1, 0)
	case 'q':
		g.running = false
	}
}

func (g *Game) movePlayer(p *Player, dx, dy int) {
	newX := p.pos.x + dx
	newY := p.pos.y + dy

	if newX >= 1 && newX < fieldWidth-1 && newY >= 1 && newY < fieldHeight-1 {
		p.pos.x = newX
		p.pos.y = newY

		if p.pos.x == g.ball.pos.x && p.pos.y == g.ball.pos.y {
			g.ball.vx = dx * 2
			g.ball.vy = dy * 2
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

	if g.ball.pos.y <= 0 || g.ball.pos.y >= fieldHeight-1 {
		g.ball.vy = -g.ball.vy
		g.ball.pos.y += g.ball.vy
	}

	goalTop := fieldHeight/2 - 2
	goalBottom := fieldHeight/2 + 2

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

	if g.ball.vx > 0 {
		g.ball.vx--
	} else if g.ball.vx < 0 {
		g.ball.vx++
	}

	if g.ball.vy > 0 {
		g.ball.vy--
	} else if g.ball.vy < 0 {
		g.ball.vy++
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
	g.ball.pos = Position{x: fieldWidth / 2, y: fieldHeight / 2}
	g.ball.vx = 0
	g.ball.vy = 0
	time.Sleep(1 * time.Second)
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
	goalTop := fieldHeight/2 - 2
	goalBottom := fieldHeight/2 + 2
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
