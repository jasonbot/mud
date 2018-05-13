package mud

import (
	"fmt"
	"io"
	"math"
	"strings"
	"unicode/utf8"

	"github.com/ahmetb/go-cursor"
	"github.com/gliderlabs/ssh"
	"github.com/mgutz/ansi"
)

// Screen represents a UI screen. For now, just an SSH terminal.
type Screen interface {
	ToggleInput()
	ToggleChat()
	ToggleCommand()
	InputActive() bool
	InCommandMode() bool
	HandleInputKey(string)
	GetChat() string
	ToggleInventory()
	InventoryActive() bool
	Render()
	Reset()
}

type sshScreen struct {
	session          ssh.Session
	builder          WorldBuilder
	user             User
	screenSize       ssh.Window
	refreshed        bool
	colorCodeCache   map[string](func(string) string)
	keyCodeMap       map[string]func()
	inputActive      bool
	inputSticky      bool
	inputText        string
	commandMode      bool
	inventoryActive  bool
	selectedCreature string
}

const allowMouseInputAndHideCursor string = "\x1b[?1003h\x1b[?25l"
const resetScreen string = "\x1bc"
const ellipsis = "…"
const hpon = "◆"
const hpoff = "◇"
const bgcolor = 232

func truncateRight(message string, width int) string {
	if utf8.RuneCountInString(message) < width {
		fmtString := fmt.Sprintf("%%-%vs", width)

		return fmt.Sprintf(fmtString, message)
	}
	return string([]rune(message)[0:width-1]) + ellipsis
}

func truncateLeft(message string, width int) string {
	if utf8.RuneCountInString(message) < width {
		fmtString := fmt.Sprintf("%%-%vs", width)

		return fmt.Sprintf(fmtString, message)
	}
	strLen := utf8.RuneCountInString(message)
	return ellipsis + string([]rune(message)[strLen-width:strLen-1])
}

func centerText(message, pad string, width int) string {
	if utf8.RuneCountInString(message) > width {
		return truncateRight(message, width)
	}
	leftover := width - utf8.RuneCountInString(message)
	left := leftover / 2
	right := leftover - left

	leftString := ""
	for utf8.RuneCountInString(leftString) <= left && utf8.RuneCountInString(leftString) <= right {
		leftString += pad
	}

	return fmt.Sprintf("%s%s%s", string([]rune(leftString)[0:left]), message, string([]rune(leftString)[0:right]))
}

// SSHString render log item for console
func (item *LogItem) SSHString(width int) string {
	formatFunc := ansi.ColorFunc(fmt.Sprintf("255:%v", bgcolor))
	boldFormatFunc := ansi.ColorFunc(fmt.Sprintf("15+b:%v", bgcolor))
	systemFunc := ansi.ColorFunc(fmt.Sprintf("230+b:%v", bgcolor))
	actionFunc := ansi.ColorFunc(fmt.Sprintf("247+b:%v", bgcolor))
	activityFunc := ansi.ColorFunc(fmt.Sprintf("230:%v", bgcolor))
	switch item.MessageType {
	case MESSAGECHAT:
		return boldFormatFunc(item.Author) + formatFunc(": "+truncateRight(item.Message, width-(2+utf8.RuneCountInString(item.Author))))
	case MESSAGESYSTEM:
		return systemFunc(centerText(item.Message, " ", width))
	case MESSAGEACTION:
		return actionFunc(truncateRight(item.Message, width))
	case MESSAGEACTIVITY:
		return activityFunc(truncateRight(item.Message, width))
	default:
		truncateRight(item.Message, width)
	}

	return truncateRight(item.Message, width)
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

	fmtString := fmt.Sprintf("%%-%vs", inputWidth-7)

	chatFunc := screen.colorFunc(fmt.Sprintf("231:%v", bgcolor))
	chat := chatFunc("SAY▶ ")
	if screen.commandMode {
		chat = chatFunc("CMD◊ ")
	}
	if screen.InputActive() {
		chatFunc = screen.colorFunc(fmt.Sprintf("0+b:%v", bgcolor-1))
	}

	fixedChat := truncateLeft(screen.inputText, int(inputWidth-7))

	inputText := fmt.Sprintf("%s%s%s", move, chat, chatFunc(fmt.Sprintf(fmtString, fixedChat)))

	io.WriteString(screen.session, inputText)
}

func (screen *sshScreen) drawBox(x, y, width, height int) {
	color := ansi.ColorCode(fmt.Sprintf("255:%v", bgcolor))

	for i := 1; i < width; i++ {
		io.WriteString(screen.session, fmt.Sprintf("%s%s─", cursor.MoveTo(y, x+i), color))
		io.WriteString(screen.session, fmt.Sprintf("%s%s─", cursor.MoveTo(y+height, x+i), color))
	}

	for i := 1; i < height; i++ {
		midString := fmt.Sprintf("%%s%%s│%%%vs│", (width - 1))
		io.WriteString(screen.session, fmt.Sprintf("%s%s│", cursor.MoveTo(y+i, x), color))
		io.WriteString(screen.session, fmt.Sprintf("%s%s│", cursor.MoveTo(y+i, x+width), color))
		io.WriteString(screen.session, fmt.Sprintf(midString, cursor.MoveTo(y+i, x), color, " "))
	}

	io.WriteString(screen.session, fmt.Sprintf("%s%s╭", cursor.MoveTo(y, x), color))
	io.WriteString(screen.session, fmt.Sprintf("%s%s╰", cursor.MoveTo(y+height, x), color))
	io.WriteString(screen.session, fmt.Sprintf("%s%s╮", cursor.MoveTo(y, x+width), color))
	io.WriteString(screen.session, fmt.Sprintf("%s%s╯", cursor.MoveTo(y+height, x+width), color))
}

func (screen *sshScreen) drawFill(x, y, width, height int) {
	color := ansi.ColorCode(fmt.Sprintf("0:%v", bgcolor))

	midString := fmt.Sprintf("%%s%%s%%%vs", (width))
	for i := 0; i <= height; i++ {
		io.WriteString(screen.session, fmt.Sprintf(midString, cursor.MoveTo(y+i, x), color, " "))
	}
}

func (screen *sshScreen) drawProgressMeter(min, max, fgcolor, bgcolor, width uint64) string {
	var blink bool
	if min > max {
		min = max
		blink = true
	}
	proportion := float64(float64(min) / float64(max))
	if math.IsNaN(proportion) {
		proportion = 0.0
	} else if proportion < 0.05 {
		blink = true
	}
	onWidth := uint64(float64(width) * proportion)
	offWidth := uint64(float64(width) * (1.0 - proportion))

	onColor := screen.colorFunc(fmt.Sprintf("%v:%v", fgcolor, bgcolor))
	offColor := onColor

	if blink {
		onColor = screen.colorFunc(fmt.Sprintf("%v+B:%v", fgcolor, bgcolor))
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
	width := (screen.screenSize.Width - x)
	fmtFunc := screen.colorFunc(fmt.Sprintf("white:%v", bgcolor))
	pos := screen.user.Location()

	CRnumberColor := screen.colorFunc(fmt.Sprintf("%v:255", bgcolor))
	CRitemColor := screen.colorFunc(fmt.Sprintf("255:%v", bgcolor))
	CRhiliteColor := screen.colorFunc(fmt.Sprintf("%v+b:255", bgcolor))

	charge, maxcharge := screen.user.Charge()

	infoLines := []string{
		centerText(screen.user.Username(), " ", width),
		centerText("", "─", width),
		truncateRight(fmt.Sprintf("%s (%v, %v)", screen.user.LocationName(), pos.X, pos.Y), width),
		truncateRight(fmt.Sprintf("Charge: %v/%v", charge, maxcharge), width),
		screen.drawProgressMeter(screen.user.HP(), screen.user.MaxHP(), 196, bgcolor, 10) + fmtFunc(truncateRight(fmt.Sprintf(" HP: %v/%v", screen.user.HP(), screen.user.MaxHP()), width-11)),
		screen.drawProgressMeter(screen.user.AP(), screen.user.MaxAP(), 208, bgcolor, 10) + fmtFunc(truncateRight(fmt.Sprintf(" AP: %v/%v", screen.user.AP(), screen.user.MaxAP()), width-11)),
		screen.drawProgressMeter(screen.user.RP(), screen.user.MaxRP(), 117, bgcolor, 10) + fmtFunc(truncateRight(fmt.Sprintf(" RP: %v/%v", screen.user.RP(), screen.user.MaxRP()), width-11)),
		screen.drawProgressMeter(screen.user.MP(), screen.user.MaxMP(), 76, bgcolor, 10) + fmtFunc(truncateRight(fmt.Sprintf(" MP: %v/%v", screen.user.MP(), screen.user.MaxMP()), width-11))}

	foundSelectedCreature := false
	hasCreatures := false
	firstID := ""
	creatures := screen.builder.World().GetCreatures(pos.X, pos.Y)
	var selectedCreatureItem *Creature
	if creatures != nil && len(creatures) > 0 {
		hasCreatures = true
		extraLines := []string{centerText(" Creatures ", "─", width)}

		for keyIndex, creature := range creatures {
			labelColumn := CRitemColor(" ")

			labelColumn = fmt.Sprintf("%2v", keyIndex+1)
			cid := creature.ID
			if creature.HP <= 0 {
				labelColumn = "✘✘"
			} else if keyIndex < 10 {
				screen.keyCodeMap[fmt.Sprintf("%v", keyIndex+1)] = func() {
					screen.selectedCreature = cid
				}

				if firstID == "" {
					firstID = creature.ID
				}
			}

			nameColumn := truncateRight(fmt.Sprintf("%s (%v/%v)  Charge: (%v/%v)",
				creature.CreatureTypeStruct.Name,
				creature.HP,
				creature.CreatureTypeStruct.MaxHP,
				creature.Charge, creature.maxCharge), width-3)

			if screen.selectedCreature == creature.ID && creature.HP > 0 {
				labelColumn = CRhiliteColor(labelColumn)
				nameColumn = CRhiliteColor("▸" + nameColumn)
				foundSelectedCreature = true
				selectedCreatureItem = creature
			} else {
				labelColumn = CRnumberColor(labelColumn)
				nameColumn = CRitemColor(" " + nameColumn)
			}

			newLine := labelColumn + nameColumn
			extraLines = append(extraLines, newLine)
		}

		infoLines = append(infoLines, extraLines...)
	}

	// Unselect creature if it's not here
	if !foundSelectedCreature {
		if screen.selectedCreature != "" {
			screen.selectedCreature = firstID
		}
	}

	if hasCreatures {
		attacks := screen.user.Attacks()
		if attacks != nil && len(attacks) > 0 {
			extraLines := []string{centerText(" Attacks ", "─", width)}

			key := 'A'
			for idx, attack := range attacks {
				attackkey := "  "
				if idx < 26 {
					if selectedCreatureItem == nil {
						attackkey = "◊◊"
					} else {
						keyString := string(key)
						attackkey = fmt.Sprintf(" %v", keyString)

						selc := selectedCreatureItem
						sela := *attack.Attack
						screen.keyCodeMap[keyString] = func() {
							selattack := screen.user.MusterAttack(sela.Name)
							if selattack != nil {
								formatString := fmt.Sprintf("Attacking %v with %v", selc.CreatureTypeStruct.Name, sela.Name)
								screen.user.Log(LogItem{Message: formatString,
									MessageType: MESSAGEACTION})
								screen.builder.Attack(selc, selattack)
							}
						}
					}
				}
				attackName := fmtFunc(truncateRight(" "+attack.Attack.String(), width-12))

				if attack.Charged {
					attackkey = CRnumberColor(attackkey)
				}

				extraLines = append(extraLines, attackkey+attackName+screen.drawProgressMeter(uint64(charge), uint64(attack.Attack.Charge), 73, bgcolor, 10))

				key++
			}

			infoLines = append(infoLines, extraLines...)
		}
	}

	infoLines = append(infoLines, centerText(" ❦ ", "─", width))

	for index, line := range infoLines {
		io.WriteString(screen.session, fmt.Sprintf("%s%s", cursor.MoveTo(2+index, x), fmtFunc(line)))
		if index+2 > int(screen.screenSize.Height) {
			break
		}
	}

	lastLine := len(infoLines) + 1
	screen.drawFill(x, lastLine+1, width, screen.screenSize.Height-(lastLine+2))
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

	for _, item := range log {
		move := cursor.MoveTo(row, screenX)
		io.WriteString(screen.session, move+item.SSHString(screenWidth-1))
		row--

		if row < y+3 {
			return
		}
	}
}

func (screen *sshScreen) ToggleInput() {
	screen.inputActive = !screen.inputActive
	screen.inputSticky = true
	screen.Render()
}

func (screen *sshScreen) ToggleChat() {
	screen.inputActive = !screen.inputActive
	screen.inputSticky = false
	screen.commandMode = false
	screen.Render()
}

func (screen *sshScreen) ToggleCommand() {
	screen.commandMode = true
	if screen.inputActive {
		screen.HandleInputKey("/")
	}
	screen.inputActive = true
	screen.Render()
}

func (screen *sshScreen) InputActive() bool {
	return !screen.inventoryActive && screen.inputActive
}

func (screen *sshScreen) InCommandMode() bool {
	return screen.commandMode
}

func (screen *sshScreen) HandleInputKey(input string) {
	if screen.inputText == "" {
		if input == "/" {
			screen.commandMode = true
			screen.inputText = ""
			input = ""
		} else if input == "!" {
			screen.commandMode = false
			screen.inputText = ""
			input = ""
		}
	}

	if !screen.inputActive {
		input := strings.ToUpper(input)
		if input == "T" ||
			input == "!" {
			screen.inputActive = true
		} else if screen.keyCodeMap != nil {
			fn, ok := screen.keyCodeMap[input]
			if ok {
				fn()
			}
		}
	} else {
		if input == "BACKSPACE" {
			if utf8.RuneCountInString(screen.inputText) > 0 {
				screen.inputText = string([]rune(screen.inputText)[0 : utf8.RuneCountInString(screen.inputText)-1])
			}
		} else if utf8.RuneCountInString(input) == 1 {
			screen.inputText += input
		}
	}

	screen.Render()
}

func (screen *sshScreen) GetChat() string {
	ct := screen.inputText
	screen.inputText = ""
	screen.inputActive = screen.inputSticky
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
	screen.keyCodeMap = make(map[string]func())

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
