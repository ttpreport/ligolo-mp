# Ligolo-mp : Tunneling like ligolo-ng, now with friends!

![Ligolo-mp Logo](doc/logo.png)

This thing is based on amazing work by [nicocha30](https://github.com/nicocha30) on [Ligolo-ng](https://github.com/nicocha30/ligolo-ng). I also borrowed quite a bit from [Sliver](https://github.com/BishopFox/sliver) codebase. Thanks, you people are amazing!

[![GPLv3](https://img.shields.io/badge/License-GPLv3-brightgreen.svg)](https://www.gnu.org/licenses/gpl-3.0)

> [!WARNING]
> Version 2.0 is almost complete and will introduce automated TUN management and a more user-friendly UI, but please use it with care: it's still in beta and is undergoing battle-testing - it's not production-ready yet.

## Introduction

**Ligolo-mp** is a more specialized version of Ligolo-ng, with client-server architecture, enabling pentesters to play with multiple concurrent tunnels collaboratively. Also, with a sprinkle of less important bells and whistles.

## Features

Everything that you love about Ligolo-ng and:

- Multiplayer
- Multiple concurrent relays
- Automatic TUN management
- Routing to the loopback of target machine (no more port forwarding)
- Listeners are now independent redirectors
- Dynamic mTLS-enabled agent binaries generation with obfuscation option
- Simplified certificate management
- Friendly terminal-based GUI

## Documentation

Please visit the [Wiki](https://github.com/ttpreport/ligolo-mp/wiki) for up-to-date information