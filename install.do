
redo-always
redo-ifchange tsproxy

# Install in local paths
sudo -n mkdir -p /usr/local/bin /usr/local/lib/systemd/user/
sudo -n cp tsproxy /usr/local/bin/tsproxy
sudo -n cp *.service /usr/local/lib/systemd/user/
systemctl --user daemon-reload
