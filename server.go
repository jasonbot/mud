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

func handleConnection(s ssh.Session) {
	io.WriteString(s, fmt.Sprintf("Hello %s\n", s.User()))

	log.Printf("Public key: %v", s.PublicKey())
	log.Printf("Environ: %v", s.Environ())
	if len(s.Command()) > 0 {
		s.Write([]byte("Commands are not supported.\n"))
		s.Close()
	}

	done := s.Context().Done()
	tick := time.Tick(1 * time.Second)
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
