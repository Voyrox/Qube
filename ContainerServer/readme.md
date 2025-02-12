# Qube Images

```bash
cargo build --release
```

```bash
sudo cp target/release/rust-file-server /usr/local/bin/rust-file-server
sudo cp rust-file-server.service /etc/systemd/system/rust-file-server.service
sudo systemctl daemon-reload
sudo systemctl start rust-file-server
sudo systemctl enable rust-file-server
sudo systemctl status rust-file-server
```