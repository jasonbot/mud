package mud

import (
	"bufio"
	"context"
	"log"
	"time"

	"github.com/gliderlabs/ssh"
)

func handleConnection(builder WorldBuilder, s ssh.Session) {
	user := builder.GetUser(s.User())
	screen := NewSSHScreen(s, builder, user)
	ctx, cancel := context.WithCancel(context.Background())

	log.Printf("Connected with %v (as %v)", s.RemoteAddr(), user)
	if len(s.Command()) > 0 {
		s.Write([]byte("Commands are not supported.\n"))
		s.Close()
	}

	done := s.Context().Done()
	tick := time.Tick(250 * time.Millisecond)
	stringInput := make(chan inputEvent, 1)
	reader := bufio.NewReader(s)

	go handleKeys(reader, stringInput, cancel)

	for {
		select {
		case inputString := <-stringInput:
			if inputString.err != nil {
				screen.Reset()
				s.Close()
				continue
			}
		case <-ctx.Done():
			cancel()
		case <-tick:
			screen.Render()
			continue
		case <-done:
			log.Printf("Disconnected %v", s.RemoteAddr())
			screen.Reset()
			s.Close()
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
