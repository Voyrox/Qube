# Qube
 A container runtime written in Rust

Add `CloneFlags::CLONE_NEWPID` later to run multiple containers
```
    unshare(CloneFlags::CLONE_NEWUTS | CloneFlags::CLONE_NEWPID | CloneFlags::CLONE_NEWNS)
```

```
You may also need a valid /etc/resolv.conf for DNS. For example:
sudo cp /etc/resolv.conf /tmp/Qube_ubuntu24/etc/resolv.conf
```

```
If you need a fully functional environment to apt-get install packages, you might need:
bash
Copy
Edit
sudo mount -o bind /dev /tmp/Qube_ubuntu24/dev
sudo mount -o bind /sys /tmp/Qube_ubuntu24/sys
sudo mount -o bind /proc /tmp/Qube_ubuntu24/proc
```

```
sudo apt-get install -y build-essential libseccomp-dev libssl-dev
```

Run a Container Shell
```
cargo build --release

sudo ./target/release/Qube run /bin/bash
```

```
sudo ./target/release/Qube list
sudo ./target/release/Qube stop <pid>
sudo ./target/release/Qube kill <pid>
```

(Optional) Copy a Real Shell
```
sudo apt-get install -y busybox

sudo cp /bin/busybox /tmp/Qube_ubuntu24/bin/ash
```

Rootfs:
```
sudo apt-get install -y debootstrap

sudo debootstrap \
     --variant=minbase \
     jammy \
     /tmp/ubuntu24rootfs \
     http://archive.ubuntu.com/ubuntu/

sudo tar -C /tmp/ubuntu24rootfs -cf ubuntu24rootfs.tar .
```

Note:
Runs on a Linux system with namespace support

todo:
```
Networking: If you want an actual network interface inside the container, add CLONE_NEWNET. Then set up veth pairs or a bridge.
Resource Limits: Use cgroups to limit CPU/memory.
Rootless: Add CLONE_NEWUSER and map UID/GIDs so it doesn’t require sudo.
Security: Use seccomp, capabilities, and AppArmor/SELinux to reduce the container’s privileges.
```