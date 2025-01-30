# Qube: A container runtime written in Rust.
[![GitHub contributors](https://img.shields.io/github/contributors/Voyrox/Qube)](https://github.com/Voyrox/Qube/graphs/contributors)
[![Github CI](https://github.com/Voyrox/Qube/actions/workflows/rust.yml/badge.svg?branch=main)](https://github.com/Voyrox/Qube/actions)

<p align="center">
  <img src="OIG4.png" width="450">
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
> $ sudo ./target/release/Qube run /bin/bash
> ```

# 📍 Status of Qube

### Manage Containers
```bash
# List running containers
sudo ./target/release/Qube list

# Stop a container
sudo ./target/release/Qube stop <pid>

# Kill a container
sudo ./target/release/Qube kill <pid>
```

### Dependencies
Install the required dependencies:

```bash
sudo apt-get install -y build-essential libseccomp-dev libssl-dev
```
### Setup
To create a root filesystem for your container:

```bash
sudo apt-get install -y debootstrap

sudo debootstrap \
    --variant=minbase \
    jammy \
    /tmp/ubuntu24rootfs \
    http://archive.ubuntu.com/ubuntu/

sudo tar -C /tmp/ubuntu24rootfs -cf ubuntu24rootfs.tar .
```

### DNS Configuration
You may need a valid `/etc/resolv.conf` for DNS:
```
sudo cp /etc/resolv.conf /tmp/Qube_ubuntu24/etc/resolv.conf
```

### Dev Notes
```bash
# To run multiple containers, add CloneFlags::CLONE_NEWPID:
unshare(CloneFlags::CLONE_NEWUTS | CloneFlags::CLONE_NEWPID | CloneFlags::CLONE_NEWNS)
```

### Roadmap
- [ ] Networking: Add CLONE_NEWNET for network interfaces inside the container.
- [ ] Rootless Containers: Add CLONE_NEWUSER and map UID/GIDs to avoid requiring sudo.
- [ ] Security: Integrate seccomp, capabilities, and AppArmor/SELinux for enhanced security.

### Contributing
Your ideas and contributions are welcome! Feel free to open issues or submit pull requests.