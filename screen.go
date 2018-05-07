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
	ToggleChat(bool)
	ChatActive() bool
	HandleChatKey(string)
	GetChat() string
	ToggleInventory()
	InventoryActive() bool
	Render()
	Reset()
}

type sshScreen struct {
	session         ssh.Session
	builder         WorldBuilder
	user            User
	screenSize      ssh.Window
	refreshed       bool
	colorCodeCache  map[string](func(string) string)
	chatActive      bool
	chatSticky      bool
	chatText        string
	inventoryActive bool
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
			for _, value := range mapArray[row] {
				fgcolor := value.FGColor
				bgcolor := value.BGColor
				mGlyph := value.Glyph

				if mGlyph == 0 {
					mGlyph = rune('?')
				}

				rowText += screen.colorFunc(fmt.Sprintf("%v:%v", fgcolor, bgcolor))(string(mGlyph))
			}

			rowText += screen.colorFunc("clear")("")
			io.WriteString(screen.session, rowText)
		}
	}
}

func (screen *sshScreen) renderChatInput() {
	inputWidth := uint32(screen.screenSize.Width/2) - 2
	move := cursor.MoveTo(screen.screenSize.Height-1, 2)

	fmtString := fmt.Sprintf("%%-%vs", inputWidth-4)

	chatFunc := screen.colorFunc(fmt.Sprintf("231:%v", bgcolor))
	chat := chatFunc("> ")
	if screen.ChatActive() {
		chatFunc = screen.colorFunc(fmt.Sprintf("0+b:%v", bgcolor-1))
	}

	fixedChat := screen.chatText
	if len(fixedChat) > int(inputWidth-4) {
		fixedChat = "…" + fixedChat[len(fixedChat)-int(inputWidth-4):len(fixedChat)-1]
	}

	chatText := fmt.Sprintf("%s%s%s", move, chat, chatFunc(fmt.Sprintf(fmtString, fixedChat)))

	io.WriteString(screen.session, chatText)
}

func (screen *sshScreen) drawBox(x, y, width, height int) {
	color := ansi.ColorCode(fmt.Sprintf("255:%v", bgcolor))

	for i := 1; i < width; i++ {
		io.WriteString(screen.session, fmt.Sprintf("%s%s─", cursor.MoveTo(y, x+i), color))
		io.WriteString(screen.session, fmt.Sprintf("%s%s─", cursor.MoveTo(y+height, x+i), color))
	}

	for i := 1; i < height; i++ {
		midString := fmt.Sprintf("%%s%%s│%%%vs│", (width - 1))
		/* io.WriteString(screen.session, fmt.Sprintf("%s%s│", cursor.MoveTo(y+i, x), color))
		io.WriteString(screen.session, fmt.Sprintf("%s%s│", cursor.MoveTo(y+i, x+width), color)) */
		io.WriteString(screen.session, fmt.Sprintf(midString, cursor.MoveTo(y+i, x), color, " "))
	}

	io.WriteString(screen.session, fmt.Sprintf("%s%s┌", cursor.MoveTo(y, x), color))
	io.WriteString(screen.session, fmt.Sprintf("%s%s└", cursor.MoveTo(y+height, x), color))
	io.WriteString(screen.session, fmt.Sprintf("%s%s┐", cursor.MoveTo(y, x+width), color))
	io.WriteString(screen.session, fmt.Sprintf("%s%s┘", cursor.MoveTo(y+height, x+width), color))
}

func (screen *sshScreen) drawVerticalLine(x, y, height int) {
	color := ansi.ColorCode(fmt.Sprintf("255:%v", bgcolor))
	for i := 1; i < height; i++ {
		io.WriteString(screen.session, fmt.Sprintf("%s%s│", cursor.MoveTo(y+i, x), color))
	}

	io.WriteString(screen.session, fmt.Sprintf("%s%s┬", cursor.MoveTo(y, x), color))
	io.WriteString(screen.session, fmt.Sprintf("%s%s┴", cursor.MoveTo(y+height, x), color))
}

func (screen *sshScreen) drawHorizontalLine(x, y, width int) {
	color := ansi.ColorCode(fmt.Sprintf("255:%v", bgcolor))
	for i := 1; i < width; i++ {
		io.WriteString(screen.session, fmt.Sprintf("%s%s─", cursor.MoveTo(y, x+i), color))
	}

	io.WriteString(screen.session, fmt.Sprintf("%s%s├", cursor.MoveTo(y, x), color))
	io.WriteString(screen.session, fmt.Sprintf("%s%s┤", cursor.MoveTo(y, x+width), color))
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
	screen.drawHorizontalLine(1, screen.screenSize.Height-2, screen.screenSize.Width/2-3)
}

func (screen *sshScreen) renderLog() {
	y := screen.screenSize.Height
	if y < 20 {
		y = 5
	} else {
		y = (y / 2) - 2
	}

	screenX := 2
	screenWidth := screen.screenSize.Width/2 - 4
	log := screen.user.GetLog()
	row := screen.screenSize.Height - 3
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

		if row < y+3 {
			return
		}
	}
}

func (screen *sshScreen) ToggleChat(sticky bool) {
	screen.chatActive = !screen.chatActive
	screen.chatSticky = sticky
	screen.Render()
}

func (screen *sshScreen) ChatActive() bool {
	return !screen.inventoryActive && screen.chatActive
}

func (screen *sshScreen) HandleChatKey(input string) {
	if input == "BACKSPACE" && len(input) > 1 {
		if len(screen.chatText) > 0 {
			screen.chatText = screen.chatText[0 : len(screen.chatText)-1]
		}
	} else if len(input) == 1 {
		screen.chatText += input
	}
}

func (screen *sshScreen) GetChat() string {
	ct := screen.chatText
	screen.chatText = ""
	screen.chatActive = screen.chatSticky
	return ct
}

func (screen *sshScreen) ToggleInventory() {
	screen.inventoryActive = !screen.inventoryActive
	screen.refreshed = false
	screen.Render()
}

func (screen *sshScreen) InventoryActive() bool {
	return screen.inventoryActive
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
	screen.renderChatInput()

	if screen.inventoryActive {
	} else {
		screen.renderLog()
	}
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
