package mud

import (
	"bufio"
	"context"
	"errors"
	"log"
	"strconv"
	"sync"
	"time"
)

type inputEvent struct {
	inputString string
	err         error
}

const (
	sOUTOFSEQUENCE = iota
	sINESCAPE
	sDIRECTIVE
	sQUESTION
)

// sleepThenReport is a timeout sequence so that if the escape key is pressed it will register
// as a keypress within a reasonable period of time with the input loop, even if the input
// state machine is in its "inside ESCAPE press listening for extended sequence" state.
func sleepThenReport(stringChannel chan<- inputEvent, myOnce *sync.Once, state *int) {
	time.Sleep(100 * time.Millisecond)

	myOnce.Do(func() {
		*state = sOUTOFSEQUENCE
		stringChannel <- inputEvent{"ESCAPE", nil}
	})
}

func handleKeys(reader *bufio.Reader, stringChannel chan<- inputEvent, cancel context.CancelFunc) {
	inputGone := errors.New("Input ended")
	inEscapeSequence := sOUTOFSEQUENCE
	var myOnce *sync.Once

	codeMap := map[rune]string{
		rune(9):   "TAB",
		rune(13):  "ENTER",
		rune(127): "BACKSPACE",
	}

	for {
		runeRead, _, err := reader.ReadRune()

		log.Printf("RUNE READ %v %v", runeRead, strconv.QuoteRune(runeRead))

		if err != nil || runeRead == 3 {
			stringChannel <- inputEvent{"", inputGone}
			cancel()
			return
		}

		if myOnce != nil {
			myOnce.Do(func() { myOnce = nil })
		}

		if inEscapeSequence == sOUTOFSEQUENCE && runeRead == 27 {
			inEscapeSequence = sINESCAPE
			myOnce = new(sync.Once)
			go sleepThenReport(stringChannel, myOnce, &inEscapeSequence)
		} else if inEscapeSequence == sINESCAPE {
			if string(runeRead) == "[" {
				inEscapeSequence = sDIRECTIVE
			} else if runeRead == 27 {
				stringChannel <- inputEvent{"ESCAPE", nil}
			} else {
				inEscapeSequence = sOUTOFSEQUENCE
				if myOnce != nil {
					myOnce.Do(func() { myOnce = nil })
				}
				stringChannel <- inputEvent{string(runeRead), nil}
			}
		} else if inEscapeSequence == sDIRECTIVE {
			switch runeRead {
			case 'A':
				stringChannel <- inputEvent{"UP", nil}
			case 'B':
				stringChannel <- inputEvent{"DOWN", nil}
			case 'C':
				stringChannel <- inputEvent{"RIGHT", nil}
			case 'D':
				stringChannel <- inputEvent{"LEFT", nil}
			case 'M':
				b, err := reader.ReadByte()
				if err != nil {
					cancel()
					return
				}

				nx, _ := reader.ReadByte()
				ny, _ := reader.ReadByte()

				pt := Point{X: uint32(nx) - 32, Y: uint32(ny) - 32}
				log.Printf("GOT IT: %32b %v @ %v", b, b, pt)

			default:
				stringChannel <- inputEvent{strconv.QuoteRune(runeRead), nil}
			}
			inEscapeSequence = sOUTOFSEQUENCE
		} else {
			if newString, ok := codeMap[runeRead]; ok {
				stringChannel <- inputEvent{newString, nil}
			} else {
				stringChannel <- inputEvent{string(runeRead), nil}
			}
		}
	}
}
