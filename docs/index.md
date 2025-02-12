---
title: Home
layout: home
---

# Qube: A container runtime written in Rust.
[![GitHub contributors](https://img.shields.io/github/contributors/Voyrox/Qube)](https://github.com/Voyrox/Qube/graphs/contributors)
[![Github CI](https://github.com/Voyrox/Qube/actions/workflows/rust.yml/badge.svg?branch=main)](https://github.com/Voyrox/Qube/actions)

<p align="center">
  <img src="./assets/images/logo.png" width="450">
</p>

## Features
- Lightweight and fast container runtime.
- Written in Rust for memory safety and performance.
- Supports basic container isolation using Linux namespaces.

## Motivation
Qube aims to provide a lightweight, secure, and efficient container runtime. Rust's memory safety and performance make it an ideal choice for implementing container runtimes. Qube is designed to be simple yet powerful, with a focus on extensibility and security.

# 🚀 Quick Start
> [!TIP]
> You can immediately set up your environment with youki on GitHub Codespaces and try it out.  
>
> [![Open in GitHub Codespaces](https://github.com/codespaces/badge.svg)](https://codespaces.new/containers/Qube?quickstart=1)
> ```console
> $ cargo build --release
> $ sudo ln -s /mnt/e/Github/Qube/target/release/Qube /usr/local/bin/Qube
> $ cp qubed.service /etc/systemd/system/qubed.service
> $ systemctl daemon-reload
> $ sudo Qube run --ports 3000 --cmd sh -c "npm i && node index.js"
> ```
