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
	session        ssh.Session
	builder        WorldBuilder
	user           User
	screenSize     ssh.Window
	renderct       uint64
	refreshed      bool
	colorCodeCache map[string](func(string) string)
}

const allowMouseInput string = "\x1b[?1003h"
const resetScreen string = "\x1bc"

func (screen *sshScreen) colorFunc(color string) func(string) string {
	_, ok := screen.colorCodeCache[color]

	if !ok {
		screen.colorCodeCache[color] = ansi.ColorFunc(color)
	}

	return screen.colorCodeCache[color]
}

func (screen *sshScreen) renderMap() {
	interfaceTools, ok := screen.builder.(SSHInterfaceTools)

	if ok {
		location := screen.user.Location()
		mapArray := interfaceTools.GetTerrainMap(location.X, location.Y, 20, 20)

		for row := range mapArray {
			rowText := cursor.MoveTo(3+row, 2)
			for _, value := range mapArray[row] {
				mGlyph := value.Glyph
				if mGlyph == 0 {
					mGlyph = rune('?')
				}
				rowText += screen.colorFunc(fmt.Sprintf("%v:%v", value.FGColor, value.BGColor))(string(mGlyph))
			}

			rowText += screen.colorFunc("clear")("|") + screen.colorFunc("red")(fmt.Sprintf("Line: %v", row))
			io.WriteString(screen.session, rowText)
		}
	}
}

func (screen *sshScreen) Render() {
	if screen.screenSize.Height < 20 || screen.screenSize.Width < 40 {
		clear := cursor.ClearEntireScreen()
		move := cursor.MoveTo(1, 1)
		io.WriteString(screen.session,
			fmt.Sprintf("%s%sScreen is too small. Make your terminal larger. (40x20 minimum)", clear, move))
		return
	}

	if !screen.refreshed {
		clear := cursor.ClearEntireScreen() + allowMouseInput
		io.WriteString(screen.session, clear)
		screen.refreshed = true
	}
	move := cursor.MoveTo(1, 1)
	color := ansi.ColorCode("blue+b")
	reset := ansi.ColorCode("reset")

	screen.renderct++

	screen.renderMap()

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

	screen := sshScreen{
		session:        session,
		builder:        builder,
		user:           user,
		screenSize:     pty.Window,
		colorCodeCache: make(map[string](func(string) string))}

	if isPty {
		go screen.watchSSHScreen(resize)
	}

	return &screen
}
