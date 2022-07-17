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

func handleConnection(builder WorldBuilder, session ssh.Session) {
	user := builder.GetUser(session.User())
	screen := NewSSHScreen(session, builder, user)
	pubKey, _ := session.Context().Value(mudPubkey).(string)
	userSSH, ok := user.(UserSSHAuthentication)

	builder.Chat(LogItem{Message: fmt.Sprintf("User %s has logged in", user.Username()), MessageType: MESSAGESYSTEM})
	user.MarkActive()
	user.Act()

	if len(session.Command()) > 0 {
		session.Write([]byte("Commands are not supported.\n"))
		session.Close()
	}

	if ok {
		if userSSH.SSHKeysEmpty() {
			userSSH.AddSSHKey(pubKey)
			log.Printf("Saving SSH key for %s", user.Username())
		} else if !userSSH.ValidateSSHKey(pubKey) {
			session.Write([]byte("This is not the SSH key verified for this user. Try another username.\n"))
			log.Printf("User %s doesn't use this key.", user.Username())
			return
		}
	}

	ctx, cancel := context.WithCancel(context.Background())

	logMessage := fmt.Sprintf("Logged in as %s via %s at %s", user.Username(), session.RemoteAddr(), time.Now().UTC().Format(time.RFC3339))
	log.Println(logMessage)
	user.Log(LogItem{Message: logMessage, MessageType: MESSAGESYSTEM})

	done := session.Context().Done()
	tick := time.Tick(500 * time.Millisecond)
	tickForOnline := time.Tick(2 * time.Second)
	stringInput := make(chan inputEvent, 1)
	reader := bufio.NewReader(session)

	go handleKeys(reader, stringInput, cancel)

	if !user.IsInitialized() {
		setupSSHUser(ctx, cancel, done, session, user, stringInput)
	}

	for {
		select {
		case inputString := <-stringInput:
			if inputString.err != nil {
				screen.Reset()
				session.Close()
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
				screen.ToggleInput()
			case "[":
				if screen.InputActive() {
					screen.HandleInputKey(inputString.inputString)
				} else if screen.InventoryActive() {
					screen.PreviousInventoryItem()
				}
			case "]":
				if screen.InputActive() {
					screen.HandleInputKey(inputString.inputString)
				} else if screen.InventoryActive() {
					screen.NextInventoryItem()
				}
			case "/":
				screen.ToggleCommand()
			case "BACKSPACE":
				if screen.InputActive() {
					screen.HandleInputKey(inputString.inputString)
				}
			case "ENTER":
				if screen.InputActive() {
					chat := screen.GetChat()
					var chatItem LogItem
					if screen.InCommandMode() {
						chatItem = LogItem{
							Author:      user.Username(),
							Message:     chat,
							MessageType: MESSAGEACTION}
						if len(chat) > 0 {
							user.Log(chatItem)
						}
					} else {
						chatItem = LogItem{
							Author:      user.Username(),
							Message:     chat,
							MessageType: MESSAGECHAT}
						if len(chat) > 0 {
							builder.Chat(chatItem)
						}
					}

					screen.Render()
				}
			default:
				screen.HandleInputKey(inputString.inputString)
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
			log.Printf("Disconnected %v@%v", user.Username(), session.RemoteAddr())
			user.Log(LogItem{Message: fmt.Sprintf("Signed off at %v", time.Now().UTC().Format(time.RFC3339)),
				MessageType: MESSAGESYSTEM})
			screen.Reset()
			session.Close()
			return
		}
	}
}

// ServeSSH runs the main SSH server loop.
func ServeSSH(listen string) {
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

	log.Printf("Starting SSH server on %v", listen)
	log.Fatal(ssh.ListenAndServe(listen, func(s ssh.Session) {
		handleConnection(builder, s)
	}, publicKeyOption, ssh.HostKeyFile(privateKey)))
}
