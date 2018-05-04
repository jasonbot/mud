package mud

import (
	"fmt"
	"io"

	"github.com/ahmetb/go-cursor"
	"github.com/gliderlabs/ssh"
	"github.com/mgutz/ansi"
)

// Screen represents a UI screen. For now, just an SSH terminal.
type Screen interface {
	Render()
	Reset()
}

type sshScreen struct {
	session    ssh.Session
	builder    WorldBuilder
	user       User
	screenSize ssh.Window
	renderct   uint64
	refreshed  bool
}

const allowMouseInput string = "\x1b[?1003h"
const resetScreen string = "\x1bc"

func (screen *sshScreen) Render() {
	if screen.screenSize.Height < 20 || screen.screenSize.Width < 80 {
		clear := cursor.ClearEntireScreen()
		move := cursor.MoveTo(1, 1)
		io.WriteString(screen.session,
			fmt.Sprintf("%s%sScreen is too small. Make your terminal larger. (80x20 minimum)", clear, move))
		return
	}

	if !screen.refreshed {
		clear := cursor.ClearEntireScreen() + allowMouseInput
		io.WriteString(screen.session, clear)
		move := cursor.MoveTo(screen.screenSize.Height, screen.screenSize.Width-10)
		io.WriteString(screen.session,
			fmt.Sprintf("%sRender %v", move, screen.renderct))
		screen.refreshed = true
	}
	move := cursor.MoveTo(2, 2)
	color := ansi.ColorCode("blue+b")
	reset := ansi.ColorCode("reset")

	screen.renderct++

	io.WriteString(screen.session,
		fmt.Sprintf("%s%sRender %v%s\n", move, color, screen.renderct, reset))
}

func (screen *sshScreen) Reset() {
	io.WriteString(screen.session, fmt.Sprintf("%s👋\n", resetScreen))
}

func (screen *sshScreen) watchSSHScreen(resizeChan <-chan ssh.Window) {
	done := screen.session.Context().Done()
	for {
		select {
		case <-done:
			return
		case win := <-resizeChan:
			screen.screenSize = win
			screen.refreshed = false
			screen.Render()
		}
	}
}

// NewSSHScreen manages the window rendering for a game session
func NewSSHScreen(session ssh.Session, builder WorldBuilder, user User) Screen {
	pty, resize, isPty := session.Pty()

	screen := sshScreen{session: session, builder: builder, user: user, screenSize: pty.Window}

	if isPty {
		go screen.watchSSHScreen(resize)
	}

	return &screen
}
