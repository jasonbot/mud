package mud

import (
	"bufio"
	"context"
	"errors"
	"strconv"
	"sync"
	"time"
)

type inputEvent struct {
	inputString string
	position    Point
	err         error
}

type inputState int

const (
	stateOUTOFSEQUENCE inputState = iota
	stateINESCAPE
	stateDIRECTIVE
	stateQUESTION
)

// sleepThenReport is a timeout sequence so that if the escape key is pressed it will register
// as a keypress within a reasonable period of time with the input loop, even if the input
// state machine is in its "inside ESCAPE press listening for extended sequence" state.
func sleepThenReport(stringChannel chan<- inputEvent, escapeCanceller *sync.Once, state *inputState) {
	time.Sleep(50 * time.Millisecond)

	escapeCanceller.Do(func() {
		*state = stateOUTOFSEQUENCE
		stringChannel <- inputEvent{"ESCAPE", Point{}, nil}
	})
}

func handleKeys(reader *bufio.Reader, stringChannel chan<- inputEvent, cancel context.CancelFunc) {
	inputGone := errors.New("Input ended")
	inEscapeSequence := stateOUTOFSEQUENCE
	var escapeCanceller *sync.Once
	emptyPoint := Point{}

	codeMap := map[rune]string{
		rune(9):   "TAB",
		rune(13):  "ENTER",
		rune(127): "BACKSPACE",
	}

	for {
		runeRead, _, err := reader.ReadRune()

		if err != nil || runeRead == 3 {
			stringChannel <- inputEvent{"", emptyPoint, inputGone}
			cancel()
			return
		}

		if escapeCanceller != nil {
			escapeCanceller.Do(func() { escapeCanceller = nil })
		}

		if inEscapeSequence == stateOUTOFSEQUENCE && runeRead == 27 {
			inEscapeSequence = stateINESCAPE
			escapeCanceller = new(sync.Once)
			go sleepThenReport(stringChannel, escapeCanceller, &inEscapeSequence)
		} else if inEscapeSequence == stateINESCAPE {
			if string(runeRead) == "[" {
				inEscapeSequence = stateDIRECTIVE
			} else if runeRead == 27 {
				stringChannel <- inputEvent{"ESCAPE", emptyPoint, nil}
			} else {
				inEscapeSequence = stateOUTOFSEQUENCE
				if escapeCanceller != nil {
					escapeCanceller.Do(func() { escapeCanceller = nil })
				}
				stringChannel <- inputEvent{string(runeRead), emptyPoint, nil}
			}
		} else if inEscapeSequence == stateDIRECTIVE {
			switch runeRead {
			case 'A':
				stringChannel <- inputEvent{"UP", emptyPoint, nil}
			case 'B':
				stringChannel <- inputEvent{"DOWN", emptyPoint, nil}
			case 'C':
				stringChannel <- inputEvent{"RIGHT", emptyPoint, nil}
			case 'D':
				stringChannel <- inputEvent{"LEFT", emptyPoint, nil}
			case 'M':
				code, err := reader.ReadByte()
				if err != nil {
					cancel()
					return
				}

				nx, _ := reader.ReadByte()
				ny, _ := reader.ReadByte()

				pt := Point{X: uint32(nx) - 32, Y: uint32(ny) - 32}

				event := ""

				switch code {
				case 32:
					event = "MOUSEDOWN"
				case 33:
					event = "MIDDLEDOWN"
				case 35:
					event = "MOUSEUP"
				case 67:
					event = "MOUSEMOVE"
				case 96:
					event = "SCROLLUP"
				case 97:
					event = "SCROLLDOWN"
				}

				if len(event) > 0 {
					stringChannel <- inputEvent{event, pt, nil}
				}
			default:
				stringChannel <- inputEvent{strconv.QuoteRune(runeRead), emptyPoint, nil}
			}
			inEscapeSequence = stateOUTOFSEQUENCE
		} else {
			if newString, ok := codeMap[runeRead]; ok {
				stringChannel <- inputEvent{newString, emptyPoint, nil}
			} else {
				stringChannel <- inputEvent{string(runeRead), emptyPoint, nil}
			}
		}
	}
}
