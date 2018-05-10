package mud

import (
	"fmt"
	"io"
	"math"

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
const ellipsis = "…"
const hpon = "◆"
const hpoff = "◇"
const bgcolor = 232

func truncateRight(message string, width int) string {
	if len(message) < width {
		fmtString := fmt.Sprintf("%%-%vs", width)

		return fmt.Sprintf(fmtString, message)
	}
	return message[0:width-1] + ellipsis
}

func truncateLeft(message string, width int) string {
	if len(message) < width {
		fmtString := fmt.Sprintf("%%-%vs", width)

		return fmt.Sprintf(fmtString, message)
	}
	return ellipsis + message[len(message)-width:len(message)-1]
}

func flushLeft(message string, width int) string {
	if len(message) < width {
		fmtString := fmt.Sprintf("%%%vs", width)

		return fmt.Sprintf(fmtString, message)
	}
	return ellipsis + message[len(message)-width:len(message)-1]
}

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
				bold := value.Bold
				mGlyph := value.Glyph

				if mGlyph == 0 {
					mGlyph = rune('?')
				}

				var fString string
				if bold {
					fString = fmt.Sprintf("%v+b:%v", fgcolor, bgcolor)
				} else {
					fString = fmt.Sprintf("%v:%v", fgcolor, bgcolor)
				}

				rowText += screen.colorFunc(fString)(string(mGlyph))
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
		fixedChat = truncateLeft(fixedChat, int(inputWidth-4))
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

	io.WriteString(screen.session, fmt.Sprintf("%s%s╭", cursor.MoveTo(y, x), color))
	io.WriteString(screen.session, fmt.Sprintf("%s%s╰", cursor.MoveTo(y+height, x), color))
	io.WriteString(screen.session, fmt.Sprintf("%s%s╮", cursor.MoveTo(y, x+width), color))
	io.WriteString(screen.session, fmt.Sprintf("%s%s╯", cursor.MoveTo(y+height, x+width), color))
}

func (screen *sshScreen) drawProgressMeter(min, max, fgcolor, bgcolor, width uint64) string {
	proportion := float64(float64(min) / float64(max))
	if math.IsNaN(proportion) {
		proportion = 0.0
	}
	onWidth := uint64(float64(width) * proportion)
	offWidth := uint64(float64(width) * (1.0 - proportion))

	onColor := screen.colorFunc(fmt.Sprintf("%v:%v", fgcolor, bgcolor))
	offColor := onColor

	if proportion < 1.5 {
		onColor = screen.colorFunc(fmt.Sprintf("%v+Bbh:%v", fgcolor, bgcolor))
	}

	if (onWidth + offWidth) > width {
		onWidth = width
		offWidth = 0
	} else if (onWidth + offWidth) < width {
		onWidth += width - (onWidth + offWidth)
	}

	on := ""
	off := ""

	for i := 0; i < int(onWidth); i++ {
		on += hpon
	}

	for i := 0; i < int(offWidth); i++ {
		off += hpoff
	}

	return onColor(on) + offColor(off)
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

func (screen *sshScreen) renderCharacterSheet() {
	x := screen.screenSize.Width/2 - 1
	width := screen.screenSize.Width - x
	fmtFunc := screen.colorFunc(fmt.Sprintf("white:%v", bgcolor))
	pos := screen.user.Location()

	infoLines := []string{
		screen.user.Username(),
		truncateRight(fmt.Sprintf("%s (%v, %v)", screen.user.LocationName(), pos.X, pos.Y), width),
		truncateRight(fmt.Sprintf("HP: %v/%v", screen.user.HP(), screen.user.MaxHP()), width-10) + screen.drawProgressMeter(screen.user.HP(), screen.user.MaxHP(), 196, bgcolor, 10),
		truncateRight(fmt.Sprintf("AP: %v/%v", screen.user.AP(), screen.user.MaxAP()), width-10) + screen.drawProgressMeter(screen.user.AP(), screen.user.MaxAP(), 208, bgcolor, 10),
		truncateRight(fmt.Sprintf("MP: %v/%v", screen.user.MP(), screen.user.MaxMP()), width-10) + screen.drawProgressMeter(screen.user.MP(), screen.user.MaxMP(), 76, bgcolor, 10),
		truncateRight(fmt.Sprintf("RP: %v/%v", screen.user.RP(), screen.user.MaxRP()), width-10) + screen.drawProgressMeter(screen.user.RP(), screen.user.MaxRP(), 117, bgcolor, 10)}

	for index, line := range infoLines {
		io.WriteString(screen.session, fmt.Sprintf("%s%s", cursor.MoveTo(2+index, x), fmtFunc(line)))
	}
}

func (screen *sshScreen) renderInventory() {
}

func (screen *sshScreen) renderLog() {
	y := screen.screenSize.Height
	if y < 20 {
		y = 5
	} else {
		y = (y / 2) - 2
	}

	screenX := 2
	screenWidth := screen.screenSize.Width/2 - 3
	log := screen.user.GetLog()
	row := screen.screenSize.Height - 3
	formatFunc := screen.colorFunc(fmt.Sprintf("255:%v", bgcolor))

	for _, item := range log {
		item = truncateRight(item, screenWidth-1)
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
			screen.Render()
		}
	} else if len(input) == 1 {
		screen.chatText += input
		screen.Render()
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
	screen.renderCharacterSheet()

	if screen.inventoryActive {
		screen.renderInventory()
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
