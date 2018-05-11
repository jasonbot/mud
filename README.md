# MUD Server
A multiplayer MUD sever for a game jam on itch.io: [Enter the (Multi-User) Dungeon](https://itch.io/jam/enterthemud)

I have a newborn and 3 days to make an MVP. **LET'S SEE IF I CAN MANAGE IT!**

## Building

You need a correctly set up Golang environment with [dep](https://github.com/golang/dep) installed.

Run make.

    make

Then run `bin/mud` from this folder.

## Connecting to Play

You need ssh installed and a key generated. This is beyond the scope of this `README` but you
[can probably follow these Github instructions](https://help.github.com/articles/generating-a-new-ssh-key-and-adding-it-to-the-ssh-agent/)
in order to get it set up on your platform of choice. From there it's a simple matter of connecting:

    ssh localhost -p 2222

assuming you're running the mud server locally.

Your terminal needs to support 256 colors. iTerm2 and Terminal.app on macOS both support 256 colors as does pretty much any terminal you can think of on Linux. Pretty sure Putty works too if you et your terminal type to `xterm-256color`. Also [here is a very detailed amount of information](https://stackoverflow.com/questions/15375992/vim-difference-between-t-co-256-and-term-xterm-256color-in-conjunction-with-tmu/15378816#15378816).

## Playing

### Game mechanics

### Strengths

Your primary and secondary strength determine how you are able to attack and defend.

For instance, a sword attack is a meelee action. Throwing a grappling hook is a range action. Casting heal is a magic action. Combination actions are things like casting fireball (magic/range), shooting an arrow (range/melee), or using an enchanted staff (melee/magic).

Note you can pick the same primary and secondary, which will greatly boost that individual strength.

**Melee**: Strength is in physical action. Hand-to-hand combat, moving large objects.

**Range**: Strength is in manipulating items from a distance. Accuracy in hitting things from far away, observing far away surroundings.

**Magic**: Strength is in non-physical magical craft. Casting defensive and healing spells.

The layout of the Melee/Range/Magic system is similar to Rock/Paper/Scissors: a Melee attack beats a Magic defense, a Magic offense trumps a Ranged defense, a Ranged offense beats a Melee defense.

### Skills

This is not fully fleshed out.

**People**: Strength is in persuasion and social skills. An understanding of the human landscape will reveal information.

**Places**: Ability to notice obscure details and a gift for exploration. An understanding of the environment leads to clever solutions.

**Things**: Ability to work with the physical world and tinker. An understanding of crafting and tools leads to engineered solutions.

### Keyboard commands

`up`, `down`, `left`, `right`: move your character in that direction.

`ctrl-C`: log off.

`tab`: toggle log/inventory view.

`esc`: toggle input mode.

`/`: activate command input mode (any input message that starts with `/` is treated as a command).

`T`: activate chat input mode (and input string that starts with `!` is treated a a chat)

> **Note:** No commands have been implemented yet.