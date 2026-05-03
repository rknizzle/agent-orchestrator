#!/bin/bash
# AWS Cloud-Init Script

# 1. Update and install base dependencies
dnf update -y
dnf install -y git-all 

# 2. Install GitHub CLI
type -p yum-config-manager >/dev/null || dnf install -y 'dnf-command(config-manager)'
dnf config-manager --add-repo https://cli.github.com/packages/rpm/gh-cli.repo
dnf install -y gh

# 3. Create Swap File (CRITICAL for 1GB RAM instances)
# This prevents the AI agents from crashing the server
dd if=/dev/zero of=/swapfile bs=128M count=16
chmod 600 /swapfile
mkswap /swapfile
swapon /swapfile
echo '/swapfile swap swap defaults 0 0' >> /etc/fstab

# 4. Install the Orchestrator Binary
# We pull the latest version directly from your GitHub Releases (ARM64 for t4g.micro)
mkdir -p /opt/orchestrator
cd /opt/orchestrator
# Note: Replace 'your-username' with your actual GitHub username below
LATEST_URL=$(curl -s https://api.github.com/repos/rknizzle/agent-orchestrator/releases/latest | grep "browser_download_url.*Linux_arm64.tar.gz" | cut -d '"' -f 4)
curl -L $LATEST_URL | tar xz

# 5. Setup Configuration
mkdir -p /root/.orchestrator
cat <<EOF > /root/.orchestrator/config.yaml
GITHUB_TOKEN: ${github_token}
projects:
  rknizzle/propermesh:
    GITHUB_PROJECT_ID: PVT_kwHOAmuybc4AgUus
    GITHUB_STATUS_FIELD_ID: PVTSSF_lAHOAmuybc4AgUuszgVe5gc
    includes:
      - .env
EOF

# 6. Clone the Target Project
# We clone it into /opt so the orchestrator can manage it
cd /opt
git clone https://x-access-token:${github_token}@github.com/rknizzle/propermesh.git

# 7. Install AI Agent (Example: Gemini)
# This assumes the gemini binary is available via a simple install
# You may need to add specific install commands for your preferred agent here
npm install -g @google/gemini-cli || true

# 8. Create Systemd Service
cat <<EOF > /etc/systemd/system/orchestrator.service
[Unit]
Description=GitHub Agent Orchestrator
After=network.target

[Service]
Type=simple
User=root
WorkingDirectory=/opt/orchestrator
ExecStart=/opt/orchestrator/orchestrator --repo-path /opt/propermesh --interval 60
Restart=always
RestartSec=10
Environment=PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/root/bin

[Install]
WantedBy=multi-user.target
EOF

# 9. Start the Service
systemctl daemon-reload
systemctl enable orchestrator
systemctl start orchestrator
