# Ligolo-mp : Tunneling like ligolo-ng, now with friends!

![Ligolo-mp Logo](doc/logo.png)

This thing is based on amazing work by [nicocha30](https://github.com/nicocha30) on [Ligolo-ng](https://github.com/nicocha30/ligolo-ng). I also borrowed quite a bit from [Sliver](https://github.com/BishopFox/sliver) codebase. Thanks, you people are amazing!

[![GPLv3](https://img.shields.io/badge/License-GPLv3-brightgreen.svg)](https://www.gnu.org/licenses/gpl-3.0)

## Table of Contents

<!-- START doctoc generated TOC please keep comment here to allow auto update -->
<!-- DON'T EDIT THIS SECTION, INSTEAD RE-RUN doctoc TO UPDATE -->

- [Introduction](#introduction)
- [Features](#features)
- [Important notes](#important-notes)
- [Terminology](#terminology)
- [Building](#building)
  - [Precompiled binaries](#precompiled-binaries)
  - [Building Ligolo-mp](#building-ligolo-mp)
- [Usage](#usage)
  - [Setup](#setup)
  - [Basic flow](#basic-flow)
  - [Accessing local ports](#accessing-local-ports)
  - [Chaining agents](#chaining-agents)
  - [Situational awareness](#situational-awareness)
  - [Misc](#misc)
- [Does it require Administrator/root access ?](#does-it-require-administratorroot-access-)
- [Caveats](#caveats)

<!-- END doctoc generated TOC please keep comment here to allow auto update -->

## Introduction

**Ligolo-mp** is a more specialized version of Ligolo-ng, with client-server architecture, enabling pentesters to play with multiple concurrent tunnels collaboratively. Also, with a sprinkle of less important bells and whistles.

## Features

Everything that you love about Ligolo-ng and:

- Multiplayer
- Multiple concurrent relays
- Routing to the loopback of target machine (no more port forwarding)
- Listeners are now independent redirectors
- Stricter agent liveness checks
- Built-in TUN management
- Dynamic mTLS-enabled agent binaries generation with obfuscation option
- Simplified certificate management

## Important notes

- This thing doesn't try to be stealthy: there are no tricky malleable profiles, no network fuckery - you will be detected. You have been warned.
- Server-side is linux-only (agents are still multi-platform, don't worry)
- Everything uses self-signed certs
- This is mostly just somehow slapped together, so use at your own risk

## Terminology

On our local machine we use *client* to connect to a *server*, that's running on the attacking machine. Then we run an *agent* on the machine we want pivot through - a target machine. To actually start pivoting, we create a *tun* and use it to start a *relay* between *server* and *agent*. We can also start a *listener* to, for example, chain connections through *agents* in cases where target machine can't directly reach our *server*.

Here's a very professional visual:

![Ligolo-mp architecture](doc/diagram.png)

## Building

### Precompiled binaries

Precompiled binaries (Windows/Linux/macOS) are available on the [Release page](https://github.com/ttpreport/ligolo-mp/releases).

### Building Ligolo-mp

Just refer to the makefile, but just for completeness sake:

```shell
# Build server
$ make assets server

# Build client
$ make client

# Build everything
$ make all
```

## Usage

### Setup

1. Put the server binary in /usr/local/bin (or wherever you prefer)
2. Create systemd service, like this one. You can change listening ports, connection pool size and so on with more flags (refer to -h), but let's stick to defaults for now

```shell
[Unit]
Description=Ligolo-mp
After=network.target
StartLimitIntervalSec=0

[Service]
Type=simple
Restart=on-failure
RestartSec=3
User=root
ExecStart=/usr/local/bin/ligolo-mp-server -daemon

[Install]
WantedBy=multi-user.target
```

2. Run the cli part of the server with the same user as you run daemon (most probably just sudo it) and create an operator or two

```shell
$ ligolo-mp-server
...
ligolo-mp » operator new --name myoperator --server 127.0.0.1:58008 --path /home/kali/
Operator 'myoperator' added. Config is saved to '/home/kali/myoperator_127.0.0.1:58008_ligolo-mp.json'
```

3. Now, on the client side, import generated config. You can import multiple configs, if you need

```shell
$ ligolo-client -import /home/kali/myoperator_127.0.0.1:58008_ligolo-mp.json
Credentials successfully imported!
```

4. Run client, choose the config you need and start pivoting!

```shell
$ ligolo-client_linux                                                             
Use the arrow keys to navigate: ↓ ↑ → ← 
Select credentials
    operator
    another_operator
  -> myoperator
    friendly_operator

--------- Agents ----------
Name:          myoperator
Server:        127.0.0.1:58008
```

### Basic flow

0. Use 'help' command, should be pretty self-explanatory

1. Create a TUN

```shell
ligolo-mp » tun new
[+] New TUN created: confident_euclid
```

2. Add routes to networks which are accessible via host that's running running ligolo agent

```shell
ligolo-mp » tun route new 10.10.2.0/24 10.10.3.0/24 10.10.4.0/32
Selected: confident_euclid
[+] Route 10.10.2.0/24@confident_euclid created

[+] Route 10.10.3.0/24@confident_euclid created

[+] Route 10.10.4.0/32@confident_euclid created
```

3. Generate an agent binary

```shell
$ agent generate --save /home/kali/agent --os linux --arch amd64 --server 127.0.0.1:11601 --obfuscate
[+] Agent binary saved to /home/kali/agent
```

4. Run agent on the target machine

```shell
# On target
$ ./agent
```

```shell
# On client
[+] Agent bold_nash@127.0.0.1/8,::1/128,192.168.2.53/24 joined

ligolo-mp »  
```

5. Start the relay

```shell
ligolo-mp » relay start
Selected: bold_nash
Selected: confident_euclid
[+] Established a tunnel with agent bold_nash@127.0.0.1/8,::1/128,192.168.2.53/24
```

That's it, your pivot is ready to use. Using this flow you can maintain as many combinations of TUNs, routes and agents as you need. 

Adding routes routine will check for overlaps in currently active TUNs and error out if that's the case. The only thing to keep in mind is that there's 1 to 1 mapping between agent and TUN, but I think there are enough railguards in place to prevent any undefined behavior.

If your agent dies, you can just restart in on the target machine and all the routes and the tunnel will be applied automatically. Also, if agent can't connect back to the server, it will retry indefinitely.

### Accessing local ports

If you need to access the services running on the same machine that's running an agent, you can use `--loopback` option for a TUN. Let me demonstrate:

0. Machine with agent bold_nash is running a service on 127.0.0.1:9999

1. Create a loopback TUN

```shell
$ ligolo-mp » tun new --loopback 
[+] New TUN created: nervous_cori
```

2. Add a route that you want to represent agent's localhost, for example

```shell
$ ligolo-mp » tun route new 10.10.10.10/24
Selected: nervous_cori
[+] Route 10.10.10.10/24@nervous_cori created
```

3. Start a relay

```shell
$ ligolo-mp » relay start 
Selected: bold_nash
Selected: nervous_cori
[+] Established a relay with agent bold_nash@127.0.0.1/8,::1/128,192.168.2.53/24
```

4. That's it. Now, accessing 10.10.10.10:9999 will route traffic to the bold_nash's 127.0.0.1:9999

### Chaining agents

If your target machine (machine#1) can't reach the server directly, but can access a machine that's running another agent (machine#2), you can use listeners to proxy the traffic. So, to implement a chain machine#1<->machine#2<->server:

1. Create a listener on machine#2's agent:

```shell
# 192.168.2.53 - bold_nash's address (you can also just use 0.0.0.0, if you want)
# 192.168.2.10 - server's address
ligolo-mp » listener new --from 192.168.2.53:1234 --to 192.168.2.10:11601
Selected: bold_nash
[+] Listener (bold_nash) from 192.168.2.53:1234 to 192.168.2.10:11601 started
```

2. Generate an agent that'll connect through the listener
```shell
# Notice the --server option, it's pointing to the listener
ligolo-mp » agent generate --save /home/kali/chained_agent --os linux --server 192.168.2.53:1234 
[+] Agent binary saved to /home/kali/chained_agent
```

3. That's it, just run an agent as usual on the machine#1.

This listener is just a TCP redirector, so you can probably find more ways to use it.

### Situational awareness

If you just joined your friend's server, you probably need to see what's going on, you can do it with `list` subcommand

* Listing active TUNs

```shell
$ ligolo-mp » tun list
┌───────────────────────────────────────────────┐
│ Active TUNs                                   │
├────────────────┬────────────────┬─────────────┤
│ ALIAS          │ ROUTES         │ IS LOOPBACK │
├────────────────┼────────────────┼─────────────┤
│ gallant_kepler │ 11.11.11.0/24  │ false       │
│                │ fe80::/64      │             │
│ nervous_cori   │ fe80::/64      │ true        │
│                │ 172.10.10.1/32 │             │
│                │ 10.11.0.0/16   │             │
│                │ 10.10.10.0/24  │             │
└────────────────┴────────────────┴─────────────┘
```

* Listing active agents

```shell
$ ligolo-mp » agent list 
┌───────────────────────────────────────────────────────────────────────────────────────────────────────┐
│ Active agents                                                                                         │
├──────────────┬─────────────────────────────────────────┬───────────────┬────────────────┬─────────────┤
│ ALIAS        │ AGENT INTERFACES                        │ CONNECTED TUN │ ROUTES         │ IS LOOPBACK │
├──────────────┼─────────────────────────────────────────┼───────────────┼────────────────┼─────────────┤
│ elated_wiles │ 127.0.0.1/8                             │ nervous_cori  │ 10.10.10.0/24  │ true        │
│              │ ::1/128                                 │               │ fe80::/64      │             │
│              │ 192.168.2.53/24                         │               │ 172.10.10.1/32 │             │
│              │                                         │               │ 10.11.0.0/16   │             │
└──────────────┴─────────────────────────────────────────┴───────────────┴────────────────┴─────────────┘
```

* Listing active listeners

```shell
$ ligolo-mp » listener list 
┌─────────────────────────────────────────────────────────────────────────────────┐
│ Active listeners                                                                │
├────────────────────────┬──────────────┬────────────────────┬────────────────────┤
│ ALIAS                  │ AGENT        │ FROM               │ TO                 │
├────────────────────────┼──────────────┼────────────────────┼────────────────────┤
│ dazzling_chandrasekhar │ elated_wiles │ 192.168.2.53:31337 │ 192.168.2.10:31337 │
└────────────────────────┴──────────────┴────────────────────┴────────────────────┘
```

### Misc

* You can enable binary obfuscation for agent generation with flag `--obfuscate`. Not sure why you'd need that, since this whole thing is not stealthy at all, but I was borrowing code from Sliver and it was there too
* There is basic certificate management available with the command `certificate`. You can use it to refresh CA cert, for example.

## Does it require Administrator/root access ?

On the *agent* side, no! Everything can be performed without administrative access.

However, on your the *server*, you need to be able to create/modify *tun* interfaces.


## Caveats

Because the *agent* is running without privileges, it's not possible to forward raw packets.
When you perform a NMAP SYN-SCAN, a TCP connect() is performed on the agent.

When using *nmap*, you should use `--unprivileged` or `-PE` to avoid false positives.

## TODO

- Change or at least refactor current agent protocol - it's a bit awkward to work with
- Certificate revokation for compromised agents
- Make TUN management more opinionated and get rid of manual management of them altogether
