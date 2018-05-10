# MUD Server
A multiplayer MUD sever for a game jam on itch.io: [Enter the (Multi-User) Dungeon](https://itch.io/jam/enterthemud)

I have a newborn and 4 days to make an MVP. **LET'S SEE IF I CAN MANAGE IT!**

## Building

You need a correctly set up Golang environment with [dep](https://github.com/golang/dep) installed.

Run make.

    make

Then run `bin/mud`.

## Connecting to Play

You need ssh installed and a key generated. This is beyond the scope of this `README` but you
[can probably follow these Github instructions](https://help.github.com/articles/generating-a-new-ssh-key-and-adding-it-to-the-ssh-agent/)
in order to get it set up on your platform of choice. From there it's a simple matter of connecting:

    ssh localhost -p 2222

assuming you're running the mud server locally.

## Playing

### Keyboard commands

`up`, `down`, `left`, `right`: move your character in that direction.

`ctrl-C`: log off.

`esc`: toggle chat mode.

`tab`: toggle log/inventory view.