package mud

import (
	"fmt"

	"github.com/ahmetb/go-cursor"
	"github.com/mgutz/ansi"
)

func internalCursorDemo() string {
	clear := cursor.ClearEntireScreen()
	move := cursor.MoveTo(0, 0)
	color := ansi.ColorCode("red+b")
	reset := ansi.ColorCode("reset")

	return fmt.Sprintf("%s%s%sRed%s\n", clear, move, color, reset)
}
