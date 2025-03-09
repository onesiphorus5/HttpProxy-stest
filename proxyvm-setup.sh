#!/bin/bash
set -e

# Variables
VM_NAME="proxy-vm"
IMG_DIR="$HOME/vm-images"
QCOW2="$IMG_DIR/$VM_NAME.qcow2"
ISO="$IMG_DIR/ubuntu-22.04.iso"
MEMORY=1024
CPUS=1
PORT=8888

# Create image directory
mkdir -p "$IMG_DIR"

# Download Ubuntu ISO if not present
if [ ! -f "$ISO" ]; then
    wget -O "$ISO" https://releases.ubuntu.com/22.04/ubuntu-22.04.3-live-server-amd64.iso
fi

# Create QCOW2 disk if not present
if [ ! -f "$QCOW2" ]; then
    qemu-img create -f qcow2 "$QCOW2" 10G
fi

# Install VM (headless, auto-install via cloud-init)
if ! virsh list --all | grep -q "$VM_NAME"; then
    virt-install \
        --name "$VM_NAME" \
        --memory "$MEMORY" \
        --vcpus "$CPUS" \
        --disk "$QCOW2" \
        --os-variant ubuntu22.04 \
        --network network=default \
        --graphics none \
        --console pty,target_type=serial \
        --import \
        --noautoconsole
fi

# Start VM if not running
if ! virsh list | grep -q "$VM_NAME"; then
    virsh start "$VM_NAME"
fi

# Wait for VM to boot (adjust timing as needed)
sleep 30

# Get VM IP (assumes default NAT network)
VM_IP=$(virsh domifaddr "$VM_NAME" | grep -oE '[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+' | head -1)
echo "VM IP: $VM_IP"

# Copy and run proxy binary (assumes SSH access setup)
scp -i ~/.ssh/id_rsa ./proxy ubuntu@"$VM_IP":/home/ubuntu/proxy
ssh -i ~/.ssh/id_rsa ubuntu@"$VM_IP" "sudo chmod +x /home/ubuntu/proxy && sudo MODE=proxy TARGET=172.17.0.2:8000 /home/ubuntu/proxy &"

echo "Proxy running at $VM_IP:$PORT"