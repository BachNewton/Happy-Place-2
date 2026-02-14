package server

import (
	"fmt"
	"io"
	"log"
	"sync"
	"unicode/utf8"

	"github.com/gliderlabs/ssh"

	"happy-place-2/internal/game"
	"happy-place-2/internal/render"
)

// SSHServer wraps the SSH listener and game loop integration.
type SSHServer struct {
	gameLoop *game.GameLoop
	addr     string
	hostKey  string
}

// NewSSHServer creates a new SSH server bound to the given address.
func NewSSHServer(addr string, hostKey string, gl *game.GameLoop) *SSHServer {
	return &SSHServer{
		gameLoop: gl,
		addr:     addr,
		hostKey:  hostKey,
	}
}

// Start begins listening for SSH connections.
func (s *SSHServer) Start() error {
	server := &ssh.Server{
		Addr: s.addr,
		Handler: func(sess ssh.Session) {
			s.handleSession(sess)
		},
	}

	// Set host key
	if err := server.SetOption(ssh.HostKeyFile(s.hostKey)); err != nil {
		return fmt.Errorf("set host key: %w", err)
	}

	log.Printf("SSH server listening on %s", s.addr)
	return server.ListenAndServe()
}

func (s *SSHServer) handleSession(sess ssh.Session) {
	// Require PTY
	ptyReq, winCh, ok := sess.Pty()
	if !ok {
		fmt.Fprintln(sess, "Error: PTY required. Use: ssh -t ...")
		return
	}

	username := sess.User()
	if username == "" {
		username = "Anonymous"
	}

	// Register with game loop (username = identity)
	playerID, renderCh := s.gameLoop.AddPlayer(username)

	log.Printf("Player connected: %s (%s)", username, playerID)
	defer func() {
		s.gameLoop.RemovePlayer(playerID)
		log.Printf("Player disconnected: %s (%s)", username, playerID)
	}()

	// Terminal dimensions
	termW := ptyReq.Window.Width
	termH := ptyReq.Window.Height
	var termMu sync.Mutex

	// Create renderer
	engine := render.NewEngine(termW, termH)

	// Setup terminal
	io.WriteString(sess, render.EnableAltScreen())
	io.WriteString(sess, render.HideCursor())
	io.WriteString(sess, render.ClearScreen())
	defer func() {
		io.WriteString(sess, render.ShowCursor())
		io.WriteString(sess, render.DisableAltScreen())
	}()

	inputCh := s.gameLoop.InputChan()
	quitCh := make(chan struct{})

	// Goroutine: read input
	go func() {
		buf := make([]byte, 64)
		for {
			n, err := sess.Read(buf)
			if err != nil {
				close(quitCh)
				return
			}
			actions := parseInput(buf[:n])
			for _, action := range actions {
				if action == game.ActionQuit {
					close(quitCh)
					return
				}
				select {
				case inputCh <- game.InputEvent{PlayerID: playerID, Action: action}:
				default:
				}
			}
		}
	}()

	// Goroutine: handle window resizes
	go func() {
		for win := range winCh {
			termMu.Lock()
			termW = win.Width
			termH = win.Height
			termMu.Unlock()
		}
	}()

	// Main render loop: read from render channel
	for {
		select {
		case <-quitCh:
			return
		case state, ok := <-renderCh:
			if !ok {
				return
			}

			termMu.Lock()
			w, h := termW, termH
			termMu.Unlock()

			// Convert game snapshots to render player info
			players := make([]render.PlayerInfo, len(state.Players))
			for i, p := range state.Players {
				players[i] = render.PlayerInfo{
					ID:    p.ID,
					Name:  p.Name,
					X:     p.X,
					Y:     p.Y,
					Color: p.Color,
				}
			}

			output := engine.Render(playerID, state.Map, players, w, h, state.Tick)
			if len(output) > 0 {
				io.WriteString(sess, output)
			}
		}
	}
}

// parseInput converts raw bytes into player actions.
// Handles WASD, arrow key escape sequences, Q, and Ctrl-C.
func parseInput(data []byte) []game.Action {
	var actions []game.Action
	i := 0
	for i < len(data) {
		// Check for escape sequences (arrow keys)
		if i+2 < len(data) && data[i] == 0x1b && data[i+1] == '[' {
			switch data[i+2] {
			case 'A':
				actions = append(actions, game.ActionUp)
			case 'B':
				actions = append(actions, game.ActionDown)
			case 'C':
				actions = append(actions, game.ActionRight)
			case 'D':
				actions = append(actions, game.ActionLeft)
			}
			i += 3
			continue
		}

		// Single byte inputs
		r, size := utf8.DecodeRune(data[i:])
		switch r {
		case 'w', 'W':
			actions = append(actions, game.ActionUp)
		case 's', 'S':
			actions = append(actions, game.ActionDown)
		case 'a', 'A':
			actions = append(actions, game.ActionLeft)
		case 'd', 'D':
			actions = append(actions, game.ActionRight)
		case 'q', 'Q':
			actions = append(actions, game.ActionQuit)
		case 3: // Ctrl-C
			actions = append(actions, game.ActionQuit)
		}
		i += size
	}
	return actions
}
