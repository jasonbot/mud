package mud

import (
	"bufio"
	"context"
	"fmt"
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

	builder.Chat(LogItem{Message: fmt.Sprintf("User %s has logged in", user.Username()), MessageType: MESSAGESYSTEM})
	user.MarkActive()

	if len(s.Command()) > 0 {
		s.Write([]byte("Commands are not supported.\n"))
		s.Close()
	}

	if ok {
		if userSSH.SSHKeysEmpty() {
			userSSH.AddSSHKey(pubKey)
			log.Printf("Saving SSH key for %s", user.Username())
		} else if !userSSH.ValidateSSHKey(pubKey) {
			s.Write([]byte("This is not the SSH key verified for this user. Try another username.\n"))
			log.Printf("User %s doesn't use this key.", user.Username())
			return
		}
	}

	ctx, cancel := context.WithCancel(context.Background())

	logMessage := fmt.Sprintf("Logged in as %s via %s at %s", user.Username(), s.RemoteAddr(), time.Now().UTC().Format(time.RFC3339))
	log.Println(logMessage)
	user.Log(LogItem{Message: logMessage, MessageType: MESSAGESYSTEM})

	done := s.Context().Done()
	tick := time.Tick(500 * time.Millisecond)
	tickForOnline := time.Tick(4 * time.Second)
	stringInput := make(chan inputEvent, 1)
	reader := bufio.NewReader(s)

	go handleKeys(reader, stringInput, cancel)

	if !user.IsInitialized() {
		setupSSHUser(ctx, cancel, done, s, user, stringInput)
	}

	for {
		select {
		case inputString := <-stringInput:
			if inputString.err != nil {
				screen.Reset()
				s.Close()
				continue
			}
			switch inputString.inputString {
			case "UP":
				builder.MoveUserNorth(user)
				screen.Render()
			case "DOWN":
				builder.MoveUserSouth(user)
				screen.Render()
			case "LEFT":
				builder.MoveUserWest(user)
				screen.Render()
			case "RIGHT":
				builder.MoveUserEast(user)
				screen.Render()
			case "TAB":
				screen.ToggleInventory()
			case "ESCAPE":
				screen.ToggleChat(true)
			case "/":
				screen.ActivateCommand()
			case "BACKSPACE":
				if screen.ChatActive() {
					screen.HandleChatKey(inputString.inputString)
				}
			case "ENTER":
				if screen.ChatActive() {
					chat := screen.GetChat()
					var chatItem LogItem
					if chat[0] == '/' {
						chatItem = LogItem{Author: user.Username(), Message: chat[1:len(chat)], MessageType: MESSAGEACTION}
					} else {
						chatItem = LogItem{Author: user.Username(), Message: chat, MessageType: MESSAGECHAT}
					}
					if len(chat) > 0 {
						builder.Chat(chatItem)
					}
					screen.Render()
				}
			default:
				if screen.ChatActive() {
					screen.HandleChatKey(inputString.inputString)
				} else if inputString.inputString == "t" || inputString.inputString == "T" {
					screen.ToggleChat(false)
				}
			}
		case <-ctx.Done():
			cancel()
		case <-tickForOnline:
			user.MarkActive()
		case <-tick:
			user.Reload()
			screen.Render()
			continue
		case <-done:
			log.Printf("Disconnected %v", s.RemoteAddr())
			user.Log(LogItem{Message: fmt.Sprintf("Signed off at %v", time.Now().UTC().Format(time.RFC3339)),
				MessageType: MESSAGESYSTEM})
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
