package mud

import (
	"bufio"
	"context"
	"log"
	"math/rand"
	"time"

	gossh "golang.org/x/crypto/ssh"

	"github.com/gliderlabs/ssh"
)

const mudPubkey = "MUD-pubkey"

func handleConnection(builder WorldBuilder, s ssh.Session) {
	user := builder.GetUser(s.User())
	screen := NewSSHScreen(s, builder, user)
	pubKey, _ := s.Context().Value(mudPubkey).(string)
	userSSH, ok := user.(UserSSHAuthentication)

	if len(s.Command()) > 0 {
		s.Write([]byte("Commands are not supported.\n"))
		s.Close()
	}

	if ok {
		if userSSH.SSHKeysEmpty() {
			userSSH.AddSSHKey(pubKey)
			log.Printf("Saving SSH key for %s", user.Username())
		} else if !userSSH.ValidateSSHKey(pubKey) {
			s.Write([]byte("This is not the SSH key authenticated for this user. Try another username.\n"))
			log.Printf("User %s doesn't have this key.", user.Username())
			return
		}
	}

	ctx, cancel := context.WithCancel(context.Background())

	log.Printf("Connected with host %v (as %v)", s.RemoteAddr(), user.Username())

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
	rand.Seed(time.Now().Unix())

	world := LoadWorldFromDB("./world.db")
	defer world.Close()
	builder := NewWorldBuilder(world)

	privateKey := makeKeyFiles()

	publicKeyOption := ssh.PublicKeyAuth(func(ctx ssh.Context, key ssh.PublicKey) bool {
		marshal := gossh.MarshalAuthorizedKey(key)
		ctx.SetValue(mudPubkey, string(marshal))
		return true
	})

	log.Println("Starting SSH server on :2222")
	log.Fatal(ssh.ListenAndServe(":2222", func(s ssh.Session) {
		handleConnection(builder, s)
	}, publicKeyOption, ssh.HostKeyFile(privateKey)))
}
