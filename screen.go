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
	refreshed      bool
	colorCodeCache map[string](func(string) string)
}

const allowMouseInputAndHideCursor string = "\x1b[?1003h\x1b[?25l"
const resetScreen string = "\x1bc"
const bgcolor = 232

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
		height := uint32(screen.screenSize.Height)
		if height < 20 {
			height = 5
		} else {
			height = (height / 2) - 2
		}
		mapArray := interfaceTools.GetTerrainMap(location.X, location.Y, uint32(screen.screenSize.Width/2)-4, height)

		for row := range mapArray {
			rowText := cursor.MoveTo(2+row, 2)
			for col, value := range mapArray[row] {
				fgcolor := value.FGColor
				bgcolor := value.BGColor
				mGlyph := value.Glyph

				if mGlyph == 0 {
					mGlyph = rune('?')
				}

				if (row == len(mapArray)/2) && (col == len(mapArray[row])/2) {
					fgcolor = 160
					bgcolor = 181
					mGlyph = rune('#')
				}

				rowText += screen.colorFunc(fmt.Sprintf("%v:%v", fgcolor, bgcolor))(string(mGlyph))
			}

			rowText += screen.colorFunc("clear")("")
			io.WriteString(screen.session, rowText)
		}
	}
}

func (screen *sshScreen) drawBox(x, y, width, height int) {
	for i := 1; i < width; i++ {
		io.WriteString(screen.session, fmt.Sprintf("%s─", cursor.MoveTo(y, x+i)))
		io.WriteString(screen.session, fmt.Sprintf("%s─", cursor.MoveTo(y+height, x+i)))
	}

	for i := 1; i < height; i++ {
		io.WriteString(screen.session, fmt.Sprintf("%s│", cursor.MoveTo(y+i, x)))
		io.WriteString(screen.session, fmt.Sprintf("%s│", cursor.MoveTo(y+i, x+width)))
	}

	io.WriteString(screen.session, fmt.Sprintf("%s┌", cursor.MoveTo(y, x)))
	io.WriteString(screen.session, fmt.Sprintf("%s└", cursor.MoveTo(y+height, x)))
	io.WriteString(screen.session, fmt.Sprintf("%s┐", cursor.MoveTo(y, x+width)))
	io.WriteString(screen.session, fmt.Sprintf("%s┘", cursor.MoveTo(y+height, x+width)))
}

func (screen *sshScreen) drawVerticalLine(x, y, height int) {
	for i := 1; i < height; i++ {
		io.WriteString(screen.session, fmt.Sprintf("%s│", cursor.MoveTo(y+i, x)))
	}

	io.WriteString(screen.session, fmt.Sprintf("%s┬", cursor.MoveTo(y, x)))
	io.WriteString(screen.session, fmt.Sprintf("%s┴", cursor.MoveTo(y+height, x)))
}

func (screen *sshScreen) drawHorizontalLine(x, y, width int) {
	for i := 1; i < width; i++ {
		io.WriteString(screen.session, fmt.Sprintf("%s─", cursor.MoveTo(y, x+i)))
	}

	io.WriteString(screen.session, fmt.Sprintf("%s├", cursor.MoveTo(y, x)))
	io.WriteString(screen.session, fmt.Sprintf("%s┤", cursor.MoveTo(y, x+width)))
}

func (screen *sshScreen) redrawBorders() {
	io.WriteString(screen.session, ansi.ColorCode(fmt.Sprintf("255:%v", bgcolor)))
	screen.drawBox(1, 1, screen.screenSize.Width-1, screen.screenSize.Height-1)
	screen.drawVerticalLine(screen.screenSize.Width/2-2, 1, screen.screenSize.Height)

	y := screen.screenSize.Height
	if y < 20 {
		y = 5
	} else {
		y = (y / 2) - 2
	}
	screen.drawHorizontalLine(1, y+2, screen.screenSize.Width/2-3)
}

func (screen *sshScreen) renderLog() {
	screenX := screen.screenSize.Width/2 - 1
	screenWidth := screen.screenSize.Width - screenX
	log := screen.user.GetLog()
	row := screen.screenSize.Height - 1
	fmtString := fmt.Sprintf("%%-%vs", screenWidth)
	formatFunc := screen.colorFunc(fmt.Sprintf("255:%v", bgcolor))

	for _, item := range log {
		if len(item) > screenWidth {
			item = item[:screenWidth-1] + "…"
		} else if len(item) < screenWidth {
			item = fmt.Sprintf(fmtString, item)
		}

		move := cursor.MoveTo(row, screenX)
		io.WriteString(screen.session, fmt.Sprintf("%s%s", move, formatFunc(item)))
		row--

		if row < 2 {
			return
		}
	}
}

func (screen *sshScreen) Render() {
	if screen.screenSize.Height < 20 || screen.screenSize.Width < 60 {
		clear := cursor.ClearEntireScreen()
		move := cursor.MoveTo(1, 1)
		io.WriteString(screen.session,
			fmt.Sprintf("%s%sScreen is too small. Make your terminal larger. (60x20 minimum)", clear, move))
		return
	}

	if !screen.refreshed {
		clear := cursor.ClearEntireScreen() + allowMouseInputAndHideCursor
		io.WriteString(screen.session, clear)
		screen.redrawBorders()
		screen.refreshed = true
	}

	screen.renderMap()
	screen.renderLog()
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
