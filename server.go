package mud

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"strconv"
	"time"

	"github.com/gliderlabs/ssh"
)

func handleConnection(w World, s ssh.Session) {
	io.WriteString(s, fmt.Sprintf("Hello %s\n", s.User()))

	user := w.GetUser(s.User())

	screen := NewSSHScreen(s, w, user)

	log.Printf("Public key: %v", s.PublicKey())
	log.Printf("Environ: %v", s.Environ())
	if len(s.Command()) > 0 {
		s.Write([]byte("Commands are not supported.\n"))
		s.Close()
	}

	io.WriteString(s, internalCursorDemo())

	done := s.Context().Done()
	tick := time.Tick(250 * time.Millisecond)
	stringInput := make(chan inputEvent, 1)
	reader := bufio.NewReader(s)

	go handleKeys(reader, stringInput)

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
			screen.Render()
			continue
		case <-done:
			log.Printf("Disconnected %v", s.RemoteAddr())
			return
		}
	}
}

// Serve runs the main server loop.
func Serve() {
	world := LoadWorldFromDB("./world.db")
	defer world.Close()

	privateKey := makeKeyFiles()

	log.Println("starting ssh server on port 2222...")
	log.Fatal(ssh.ListenAndServe(":2222", func(s ssh.Session) {
		handleConnection(world, s)
	}, ssh.HostKeyFile(privateKey)))
}
