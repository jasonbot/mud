# MUD Server
A multiplayer MUD sever for a game jam on itch.io: [Enter the (Multi-User) Dungeon](https://itch.io/jam/enterthemud)

I have a newborn and had about 12 days to make an MVP. I mostly succeeded?

## Building

You need a correctly set up Golang environment over 1.11; this proejct uses `go mod`.

Run make.

    make

Then run `bin/mud` from this folder.

# Connecting to Play

## Overview

This MUD is a terminal-based SSH server. You need an ssh client installed and a private key generated. This is beyond the scope of this `README`, but I'll try to set you in the right direction.

## Connecting with macOS/Linux

You [can probably follow these instructions](https://help.github.com/articles/generating-a-new-ssh-key-and-adding-it-to-the-ssh-agent/)
in order to get it set up on your platform of choice. From there it's a simple matter of connecting:

    ssh localhost -p 2222

assuming you're running the mud server locally.

Your terminal needs to support 256 colors and utf-8 encoding. iTerm2 and Terminal.app on macOS both support 256 colors as does pretty much any terminal you can think of on Linux. Putty works too if you et your terminal type to `xterm-256color`. Also [here is a very detailed amount of information on terminal types](https://stackoverflow.com/questions/15375992/vim-difference-between-t-co-256-and-term-xterm-256color-in-conjunction-with-tmu/15378816#15378816) if needed.

## Connecting with Windows

You'll need Putty and PuttyGen. [Follow the instructions here](https://system.cs.kuleuven.be//cs/system/security/ssh/setupkeys/putty-with-key.html) for how to make a key to connect.

## Usernames

You sign in with whatever username you used to log into the server. You now *own* this username on the server and nobody else can use it. No passwords! How nice! Hooray for encryption. You can also claim other usernames by logging in as other users; e.g. `ssh "Another User"@localhost -p 2222`.

# Scaling

This thing appears to just sip ram (idling at approx 35 megs with three users conencted on my MacBook Pro). Go as a language was designed to handle networked servers extremely well so I don't see why a local server on modest hardware wouldn't be able to host a good hundred or so users online at at time.

# Playing

## Game mechanics

### Strengths

Your primary and secondary strength determine how you are able to attack and defend.

For instance, a sword attack is a meelee action. Throwing a grappling hook is a range action. Casting heal is a magic action. Combination actions are things like casting fireball (magic/range), shooting an arrow (range/melee), or using an enchanted staff (melee/magic).

Note you can pick the same primary and secondary, which will greatly boost that individual strength.

**Melee**: Strength is in physical action. Hand-to-hand combat, moving large objects.

**Range**: Strength is in manipulating items from a distance. Accuracy in hitting things from far away, observing far away surroundings.

**Magic**: Strength is in non-physical magical craft. Casting defensive and healing spells.

The layout of the Melee/Range/Magic system is similar to Rock/Paper/Scissors: a Melee attack beats a Magic defense, a Magic offense trumps a Ranged defense, a Ranged offense beats a Melee defense.

### Skills

This is not fully fleshed out. Ignore for now, subject to major changes.

## Battle

You are equipped with *charge points* based on your level. Every second one charge point renews; and when your charge points are full every 5 seconds your HP will begin to restore itself. Charge points reset to zero every time you act. Moving, attacking, and changing equipment are all considering acting.

You're equipped with attacks based on the strengths you chose when starting your character and may be given additional items/buffs based on class.

# Keyboard commands

`up`, `down`, `left`, `right`: move your character in that direction.

`ctrl-c`: log off.

`tab`: toggle log/inventory view.

`esc`: toggle input mode.

`/`: activate command input mode (any input message that starts with `/` is treated as a command).

`t`: activate chat input mode (any input string that starts with `!` is treated as a chat)

> **Note:** No commands have been implemented yet.