#!/bin/bash
set -e

# Variables
VM_NAME="proxy-vm"
IMG_DIR="/tmp/vm-images"
QCOW2="$IMG_DIR/$VM_NAME.qcow2"
ISO="$IMG_DIR/ubuntu-22.04.iso"
MEMORY=2048
CPUS=1
PORT=8888
SERVER_IP="$1"  # Pass server IP as argument

# Create image directory
mkdir -p "$IMG_DIR"

# Download Ubuntu ISO if not present
if [ ! -f "$ISO" ]; then
    wget -O "$ISO" https://releases.ubuntu.com/jammy/ubuntu-22.04.5-live-server-amd64.iso
fi

# Create QCOW2 disk if not present
if [ ! -f "$QCOW2" ]; then
    qemu-img create -f qcow2 "$QCOW2" 10G
fi

# Install VM if not exists (simplified, assumes manual cloud-init setup for SSH)
if ! virsh list --all | grep -q "$VM_NAME"; then
    virt-install \
        --name "$VM_NAME" \
        --ram "$MEMORY" \
        --vcpus "$CPUS" \
        --disk path="$QCOW2",size=10,format=qcow2 \
        --os-variant ubuntu22.04 \
        --network network=default \
        --graphics none \
        --console pty,target_type=serial \
        --cdrom "$ISO" \
        --noautoconsole
fi

# Start VM if not running
if ! virsh list | grep -q "$VM_NAME"; then
    virsh start "$VM_NAME"
fi

# Wait for VM to boot
sleep 60

# Get VM IP
VM_IP=$(virsh domifaddr "$VM_NAME" | grep -oE '[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+' | head -1)
if [ -z "$VM_IP" ]; then
    echo "Failed to get VM IP"
    exit 1
fi
echo "VM IP: $VM_IP"

# Build proxy binary locally and copy to VM
go build -o proxy_exec main.go
scp -i ~/.ssh/id_rsa ./proxy_exec ubuntu@"$VM_IP":/home/ubuntu/proxy
ssh -i ~/.ssh/id_rsa ubuntu@"$VM_IP" "sudo chmod +x /home/ubuntu/proxy && sudo MODE=proxy TARGET=$SERVER_IP:8000 /home/ubuntu/proxy &"

echo "Proxy running at $VM_IP:$PORT"