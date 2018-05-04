package mud

import (
	"bufio"
	"log"
	"strconv"
	"time"

	"github.com/gliderlabs/ssh"
)

func handleConnection(builder WorldBuilder, s ssh.Session) {
	user := builder.GetUser(s.User())
	screen := NewSSHScreen(s, builder, user)

	log.Printf("Connected with %v (as %v)", s.RemoteAddr(), user)
	if len(s.Command()) > 0 {
		s.Write([]byte("Commands are not supported.\n"))
		s.Close()
	}

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
	builder := NewWorldBuilder(world)

	privateKey := makeKeyFiles()

	log.Println("Starting SSH server on :2222")
	log.Fatal(ssh.ListenAndServe(":2222", func(s ssh.Session) {
		handleConnection(builder, s)
	}, ssh.HostKeyFile(privateKey)))
}
