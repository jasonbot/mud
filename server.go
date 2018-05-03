package mud

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"log"
	"strconv"
	"time"
)

import "github.com/gliderlabs/ssh"

type inputEvent struct {
	inputString string
	err         error
}

const (
	OUTOFSEQUENCE = iota
	INESCAPE
	DIRECTIVE
)

func handleKeys(s ssh.Session, stringChannel chan<- inputEvent) {
	reader := bufio.NewReader(s)
	inputGone := errors.New("Input ended")
	inEscapeSequence := OUTOFSEQUENCE

	for {
		runeRead, len, err := reader.ReadRune()

		if err != nil || runeRead == 3 {
			log.Printf("Leaving byte handler err: %v", err)
			stringChannel <- inputEvent{"", inputGone}
		}
		log.Printf("In byte handler rune: %v len: %v", strconv.QuoteRune(runeRead), len)

		if inEscapeSequence == OUTOFSEQUENCE && runeRead == 27 {
			inEscapeSequence = INESCAPE
		} else if inEscapeSequence == INESCAPE {
			if string(runeRead) == "[" {
				inEscapeSequence = DIRECTIVE
			} else {
				inEscapeSequence = OUTOFSEQUENCE
				stringChannel <- inputEvent{string(runeRead), nil}
			}
		} else if inEscapeSequence == DIRECTIVE {
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
			inEscapeSequence = OUTOFSEQUENCE
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
			log.Printf("Got string %v", inputString)
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
