package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
)

type Dawg struct {
	parent *Dawg
	letter rune
	word bool
	childs map[rune]*Dawg
}

type Board struct {
	hand map[rune]int
	board [15][15]rune
}

type Move struct {
	x int
	y int
	down bool
	letters []rune
}

func readDawg(file string) Dawg {
	f, err := os.Open(file)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	root := Dawg{nil, ' ', false, make(map[rune]*Dawg)}

	for scanner.Scan() {
		var head *Dawg = &root
		for _, char := range scanner.Text() {
			_, exists := head.childs[char]
			if !exists {
				parent := head
				child := Dawg{parent, char, false, make(map[rune]*Dawg)}
				head.childs[char] = &child
			}
			head = head.childs[char]
		}
		head.word = true;
	}

	if err := scanner.Err(); err != nil {
		log.Fatal(err)
	}

	return root
}

func (head *Dawg) verify(word string) bool {
	for _, char := range word {
		_, exists := head.childs[char]
		if !exists {
			return false
		}
		head = head.childs[char]
	}
	return head.word
}

func onBoard(xs ...int) bool {
	for _, x := range xs {
		if x < 0 || 15 <= x {
			return false
		}
	}
	return true
}

func (b *Board) trans() {
	for x := 0; x < 15; x++ {
		for y := 0; y < 15; y++ {
			b.board[x][y], b.board[y][x] = b.board[y][x], b.board[x][y]
		}
	}
}

func readBoard() Board {
	var b Board
	reader := bufio.NewReader(os.Stdin)
	var line string
	for len(line) <= 15 * 15 {
		line, _ = reader.ReadString('\n')
	}

	for x := 0; x < 15; x++ {
		for y := 0; y < 15; y++ {
			b.board[x][y] = rune(line[x + y * 15])
		}
	}

	b.hand = make(map[rune]int)
	for i := 0; i < int(line[15 * 15] - '0'); i++ {
		b.hand[rune(line[15 * 15 + 1 + i])]++
	}

	return b
}

func (d *Dawg) getCols(b *Board) (cols [][]map[rune]int) {
	cols = make([][]map[rune]int, 15)
	for i, _ := range cols {
		cols[i] = make([]map[rune]int, 15)
	}
	for x, col := range b.board[:] {
		for y, ch := range col[:] {
			cols[x][y] = make(map[rune]int)
			if ch != ' ' { // Letter already on the board
				cols[x][y][ch] = 1;
				cols[x][y]['A'] = 0; // Anchor but not End
				cols[x][y]['S'] = 0; // Static --- can't change
			} else  {
				cols[x][y]['E'] = 0 // This tile could be blank (i.e. be the end of the word)
				if !onBoard(x - 1) || b.board[x - 1][y] == ' ' {
					if !onBoard(x + 1) || b.board[x + 1][y] == ' ' { // Free floating
						for char, _ := range b.hand {
							cols[x][y][char] = 1
						}
						continue
					}
				} // Tiles above and below
				cols[x][y]['A'] = 0 // If used, this will be an anchor
				var mx int // Find start of word
				for mx = 0; onBoard(x - mx - 1) && b.board[x - mx - 1][y] != ' '; mx++ {}
				word := make([]rune, 0)
				for l := 0; onBoard(x - mx + l) && (b.board[x - mx + l][y] != ' ' || l == mx); l++ {
					word = append(word, b.board[x - mx + l][y])
				}
				for char, _ := range b.hand {
					word[mx] = char
					if d.verify(string(word)) {
						cols[x][y][char] = 1
					}
				}
			}
		}
	}
	cols[7][7]['A'] = 0; // Middlemost tile is always an anchor
	return
}

func (d *Dawg) getRowsAndCols(b *Board) (rows [][]map[rune]int, cols [][]map[rune]int) {
	cols = d.getCols(b)
	b.trans()
	rows = d.getCols(b)
	b.trans()
	return
}

func (d *Dawg) findAllMoves(b *Board) []Move {
	movech := make(chan Move) // "valid" moves
	threadCount := make(chan int) // # of running goroutines

	var move func(*Move, bool, map[rune]int, *Dawg, []map[rune]int)
	move = func(m *Move, anchored bool, hand map[rune]int, head *Dawg, remaining []map[rune]int) {
		canEnd := true
		if len(remaining) > 0 {
			_, anchor := remaining[0]['A']
			anchored = anchor || anchored

			for char, _ := range remaining[0] {
				n, exists := hand[char]
				if !exists || n == 0 {
					_, exists = remaining[0]['S']
					if !exists {
						continue
					}
				}
				_, exists = head.childs[char]
				if !exists {
					continue
				}

				newHand := make(map[rune]int)
				for k, v := range hand {
					newHand[k] = v
				}
				newHand[char]-- // add wildcards here

				newHead := head.childs[char]

				newRemaining := remaining[1:]

				threadCount <- 1
				go move(m, anchored, newHand, newHead, newRemaining)
			}

			_, canEnd = remaining[0]['E']
		}

		if head.word && anchored && canEnd {
			move := *m
			move.letters = make([]rune, 0)
			tail := head
			for tail.parent != nil {
				move.letters = append([]rune{tail.letter}, move.letters...)
				tail = tail.parent
			}
			for i, _ := range move.letters {
				x := move.x
				y := move.y
				if move.down {
					y += i
				} else {
					x += i
				}
				if b.board[x][y] == move.letters[i] {
					move.letters[i] = '_'
				}
			}
			movech <- move
		}

		threadCount <- -1
	}

	rows, cols := d.getRowsAndCols(b)

	// Close the channel once all the searching is done
	go func() {
		i := 0
		for {
			i += <-threadCount
			if i == 0 {
				close(movech)
				return
			}
		}
	}()

	for i := 0; i < 15; i++ {
		for j := 0; j < 15; j++ {
			m1 := Move{i, j, true, nil}
			m2 := Move{j, i, false, nil}
			threadCount <- 2
			go move(&m1, false, b.hand, d, cols[i][j:])
			go move(&m2, false, b.hand, d, rows[i][j:])
		}
	}

	moves := make([]Move, 0)
	for move := range movech {
		moves = append(moves, move)
	}

	return moves
}

func (m *Move) String() string {
	x := '0' + rune(m.x)
	y := '0' + rune(m.y)
	d := 'a'
	if m.down {
		d = 'd'
	}
	l := '0' + rune(len(m.letters))
	return string(x) + string(y) + string(d) + string(l) + string(m.letters)
}

func main() {
	dict := readDawg("words.txt")
	board := readBoard()
	moves := dict.findAllMoves(&board)
	for _, move := range moves {
		fmt.Println(move.String())
	}
}
