redo-always

# Install in local paths
sudo -n mkdir -p /usr/local/bin /usr/local/lib/systemd/user/
sudo -n cp code-server-kill /usr/local/bin
sudo -n cp *.service *.socket *.timer /usr/local/lib/systemd/user/
systemctl --user daemon-reload
