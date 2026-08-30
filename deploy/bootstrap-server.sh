#!/usr/bin/env bash
# ══════════════════════════════════════════════════════════════════════════
# One-time server preparation. Run once on a fresh Ubuntu 22.04/24.04 host:
#
#   scp deploy/bootstrap-server.sh ubuntu@<server-ip>:~
#   ssh ubuntu@<server-ip> 'bash bootstrap-server.sh'
#
# Installs Docker, opens 80/443, creates /opt/happyfeet, and hardens SSH.
# Idempotent — safe to re-run.
# ══════════════════════════════════════════════════════════════════════════
set -euo pipefail

APP_DIR=/opt/happyfeet
say() { printf '\n\033[1;36m── %s\033[0m\n' "$*"; }

[ "$(id -u)" -eq 0 ] && { echo "Run as a normal sudo user, not root."; exit 1; }

say "Updating packages"
sudo apt-get update -qq
sudo DEBIAN_FRONTEND=noninteractive apt-get upgrade -yqq

say "Installing Docker"
if ! command -v docker >/dev/null 2>&1; then
	curl -fsSL https://get.docker.com | sudo sh
	sudo usermod -aG docker "$USER"
	echo "NOTE: log out and back in for docker group membership to apply."
else
	echo "already installed: $(docker --version)"
fi

say "Enabling Docker at boot"
sudo systemctl enable --now docker

say "Creating $APP_DIR"
sudo mkdir -p "$APP_DIR"
sudo chown "$USER:$USER" "$APP_DIR"

say "Opening ports 80 and 443"
# Oracle Cloud images ship with a restrictive iptables ruleset that ignores ufw,
# so punch through both. Skipping this is the single most common reason a fresh
# Oracle VM appears unreachable even though everything is running.
if command -v ufw >/dev/null 2>&1; then
	sudo ufw allow 22/tcp   >/dev/null 2>&1 || true
	sudo ufw allow 80/tcp   >/dev/null 2>&1 || true
	sudo ufw allow 443/tcp  >/dev/null 2>&1 || true
	sudo ufw --force enable >/dev/null 2>&1 || true
fi
if sudo iptables -L INPUT -n 2>/dev/null | grep -q REJECT; then
	sudo iptables -I INPUT 6 -p tcp --dport 80  -j ACCEPT || true
	sudo iptables -I INPUT 6 -p tcp --dport 443 -j ACCEPT || true
	sudo iptables -I INPUT 6 -p udp --dport 443 -j ACCEPT || true
	sudo netfilter-persistent save >/dev/null 2>&1 \
		|| sudo sh -c 'iptables-save > /etc/iptables/rules.v4' 2>/dev/null || true
	echo "iptables updated"
fi
echo "REMINDER: also allow 80/443 in the Oracle Cloud console under"
echo "          Networking → Virtual Cloud Network → Security Lists → Ingress Rules."

say "Adding swap"
# 24 GB of RAM makes swap unlikely to be touched, but a Postgres autovacuum
# spike with no swap means the OOM killer picks a victim instead.
if ! sudo swapon --show | grep -q .; then
	sudo fallocate -l 4G /swapfile
	sudo chmod 600 /swapfile
	sudo mkswap /swapfile >/dev/null
	sudo swapon /swapfile
	echo '/swapfile none swap sw 0 0' | sudo tee -a /etc/fstab >/dev/null
	echo "4 GB swap added"
else
	echo "swap already present"
fi

say "Hardening SSH"
# Deploys authenticate with a key; leaving passwords enabled leaves the door
# open to credential stuffing against a public IP.
sudo sed -i 's/^#\?PasswordAuthentication.*/PasswordAuthentication no/' /etc/ssh/sshd_config
sudo sed -i 's/^#\?PermitRootLogin.*/PermitRootLogin no/'                /etc/ssh/sshd_config
sudo systemctl reload ssh 2>/dev/null || sudo systemctl reload sshd

say "Enabling unattended security updates"
sudo DEBIAN_FRONTEND=noninteractive apt-get install -yqq unattended-upgrades >/dev/null
sudo dpkg-reconfigure -f noninteractive unattended-upgrades >/dev/null 2>&1 || true

say "Done"
cat <<EOF

Next:
  1. Copy the deploy key's PUBLIC half into ~/.ssh/authorized_keys
  2. Copy deploy/.env.prod.example to $APP_DIR/.env.prod, fill it in, chmod 600
  3. Point your DNS A records at $(curl -fsS --max-time 5 ifconfig.me 2>/dev/null || echo '<this server IP>')
  4. Push to main — GitHub Actions takes it from there

Host key for the DEPLOY_KNOWN_HOSTS secret:
EOF
ssh-keyscan -H "$(curl -fsS --max-time 5 ifconfig.me 2>/dev/null || hostname -I | awk '{print $1}')" 2>/dev/null || true
