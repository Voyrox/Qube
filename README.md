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
> $ sudo ln -s /mnt/e/Github/Qube/target/release/Qube /usr/local/bin/Qube
> $ cp qubed.service /etc/systemd/system/qubed.service
> $ systemctl daemon-reload
> $ sudo Qube run --ports 3000 --cmd sh -c "npm i && node index.js"
> ```

# 📍 Status of Qube

### Manage Containers
- Run a container

  Registers a container (with a placeholder PID) and starts it automatically via the daemon.
  ```bash
  sudo Qube run -cmd sh -c "<cmd>"
  # e.g.
  sudo Qube run --ports 3000 --cmd sh -c "npm i && node index.js"
  ```
  
<image src="./images/image.png" style="display: block;margin-left: auto;margin-right: auto;">

- List running containers

  Displays all tracked containers, along with their PIDs, uptime, and status.
  ```bash
  sudo Qube list
  ```
  
- Stop a container
  Immediately Stops a container by sending it a SIGKILL.

  ```bash
  sudo Qube stop <pid|container_name>
  ```

- Start a container
  Starts a stopped container.

  ```bash
  sudo Qube start <pid|container_name>
  ```

- Eval a container
  
  Allows you to attach to a container (by name or PID) and run commands as root inside it.
WARNING: Running commands as root inside a container may alter its configuration and pose security risks. Use with caution!

  ```bash
  # Launch an interactive shell in the container:
  sudo Qube eval <container_name|pid>

  # Execute a specific command as root in the container:
  sudo Qube eval <container_name|pid> [command]
  ```

- View container info
  Shows detailed information about a container, such as its name, PID, working directory, command, timestamp, and uptime.

  ```bash
  sudo Qube info <container_name|pid>
  ```
- Snapshot a container
  Creates a snapshot (a compressed tarball) of the container’s filesystem. The snapshot is stored in the container's working directory.

  ```bash
  sudo Qube snapshot <container_name|pid>
  ```

### Dependencies
Install the required dependencies:

```bash
sudo apt-get install -y build-essential libseccomp-dev libssl-dev tar
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
```bash
sudo cp /etc/resolv.conf /tmp/Qube_ubuntu24/etc/resolv.conf
```

### Dev Notes

### Roadmap
- [ ] Networking: Add CLONE_NEWNET for network interfaces inside the container.
- [ ] Rootless Containers: Add CLONE_NEWUSER and map UID/GIDs to avoid requiring sudo.
- [ ] Security: Integrate seccomp, capabilities, and AppArmor/SELinux for enhanced security.

### Contributing
Your ideas and contributions are welcome! Feel free to open issues or submit pull requests.