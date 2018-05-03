package mud

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"log"
	"strconv"
	"sync"
	"time"
)

import "github.com/gliderlabs/ssh"

type inputEvent struct {
	inputString string
	err         error
}

const (
	sOUTOFSEQUENCE = iota
	sINESCAPE
	sDIRECTIVE
)

// sleepThenReport is a timeout sequence so that if the escape key is pressed it will register
// as a keypress within a reasonable period of time with the input loop, even if the input
// state machine is in its "inside ESCAPE press listening for extended sequence" state.
func sleepThenReport(stringChannel chan<- inputEvent, myOnce *sync.Once, state *int) {
	time.Sleep(100 * time.Millisecond)

	myOnce.Do(func() {
		*state = sOUTOFSEQUENCE
		stringChannel <- inputEvent{string(rune(27)), nil}
	})
}

func handleKeys(s ssh.Session, stringChannel chan<- inputEvent) {
	reader := bufio.NewReader(s)
	inputGone := errors.New("Input ended")
	inEscapeSequence := sOUTOFSEQUENCE
	var myOnce *sync.Once

	for {
		runeRead, _, err := reader.ReadRune()

		if err != nil || runeRead == 3 {
			stringChannel <- inputEvent{"", inputGone}
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
				stringChannel <- inputEvent{string(rune(27)), nil}
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
			default:
				stringChannel <- inputEvent{strconv.QuoteRune(runeRead), nil}
			}
			inEscapeSequence = sOUTOFSEQUENCE
		} else {
			stringChannel <- inputEvent{string(runeRead), nil}
		}
	}
}

func handleConnection(s ssh.Session) {
	io.WriteString(s, fmt.Sprintf("Hello %s\n", s.User()))

	log.Printf("Public key: %v", s.PublicKey())
	log.Printf("Environ: %v", s.Environ())
	log.Printf("Command: %v", s.Command())

	done := s.Context().Done()
	tick := time.Tick(1 * time.Second)
	stringInput := make(chan inputEvent, 1)

	go handleKeys(s, stringInput)

	for {
		select {
		case inputString := <-stringInput:
			log.Printf("Got string s: %v err: %v", strconv.Quote(inputString.inputString), inputString.err)
			if inputString.err != nil {
				log.Printf("Input error: %v", inputString.err)
				s.Close()
				continue
			}
		case <-tick:
			continue
		case <-done:
			log.Printf("Disconnected %v", s.RemoteAddr())
			return
		}
	}
}

// Serve runs the main server loop.
func Serve() {

	privateKey := makeKeyFiles()

	log.Println("starting ssh server on port 2222...")
	log.Fatal(ssh.ListenAndServe(":2222", handleConnection, ssh.HostKeyFile(privateKey)))
}
