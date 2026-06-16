#!/usr/bin/env bash
# setup-vm.sh — instala deps Arch + crea VM Ubuntu 24.04 lista para SSH.
# Todo de un saque. Al final imprime la IP + comando ssh listo para copiar.
#
# Uso:
#   sudo ./setup-vm.sh           # full setup (deps + VM)
#   sudo ./setup-vm.sh --destroy # destruye VM (preserva base image)
#
# Idempotente: re-correr no falla si ya existe.

set -euo pipefail

VM_NAME="domain-vps-test"
VM_RAM=4096
VM_VCPUS=2
VM_DISK=15
UBUNTU_CODENAME="noble"
IMG_DIR="/var/lib/libvirt/images"
BASE_IMG="$IMG_DIR/ubuntu-24.04-cloud.qcow2"
BASE_IMG_URL="https://cloud-images.ubuntu.com/${UBUNTU_CODENAME}/current/${UBUNTU_CODENAME}-server-cloudimg-amd64.img"
VM_DISK_PATH="$IMG_DIR/${VM_NAME}.qcow2"
CI_ISO="$IMG_DIR/${VM_NAME}-cloudinit.iso"

REAL_USER="${SUDO_USER:-${USER}}"
REAL_HOME=$(getent passwd "$REAL_USER" | cut -d: -f6)
SSH_PRIV="$REAL_HOME/.ssh/id_ed25519"
SSH_PUB="$SSH_PRIV.pub"

BOLD=$'\033[1m'; RESET=$'\033[0m'
GREEN=$'\033[32m'; YELLOW=$'\033[33m'; RED=$'\033[31m'; DIM=$'\033[2m'
step() { echo ""; echo "${BOLD}==> $1${RESET}"; }
ok()   { echo "${GREEN}    ✓${RESET} $1"; }
warn() { echo "${YELLOW}    !${RESET} $1"; }
fail() { echo "${RED}    ✗${RESET} $1" >&2; }

[[ $EUID -ne 0 ]] && { fail "Necesita root. Corré con sudo."; exit 1; }

# ----------------------------------------------------------------------------
# DESTROY mode
# ----------------------------------------------------------------------------
if [[ "${1:-}" == "--destroy" ]]; then
  step "Destruyendo VM $VM_NAME"
  virsh -c qemu:///system destroy "$VM_NAME" 2>/dev/null || true
  virsh -c qemu:///system undefine "$VM_NAME" --nvram 2>/dev/null || true
  rm -f "$VM_DISK_PATH" "$CI_ISO"
  ok "VM destruida (base image preservada en $BASE_IMG)"
  exit 0
fi

# ----------------------------------------------------------------------------
# 1) Deps
# ----------------------------------------------------------------------------
step "1/7  Verificando deps de Arch"
PKGS=(libvirt qemu-base virt-install dnsmasq libisoburn edk2-ovmf wget)
MISSING=()
for p in "${PKGS[@]}"; do
  pacman -Qq "$p" >/dev/null 2>&1 || MISSING+=("$p")
done
if [[ ${#MISSING[@]} -gt 0 ]]; then
  warn "faltan: ${MISSING[*]}"
  echo "    instalando..."
  pacman -S --noconfirm --needed "${MISSING[@]}"
fi
ok "deps OK"

# ----------------------------------------------------------------------------
# 2) libvirtd
# ----------------------------------------------------------------------------
step "2/7  libvirtd"
systemctl enable --now libvirtd >/dev/null 2>&1
systemctl is-active --quiet libvirtd && ok "libvirtd corriendo" || { fail "libvirtd no arranca"; exit 1; }

# ----------------------------------------------------------------------------
# 3) Network default
# ----------------------------------------------------------------------------
step "3/7  Network default de libvirt"
if ! virsh -c qemu:///system net-info default >/dev/null 2>&1; then
  cat > /tmp/default-net.xml <<XML
<network>
  <name>default</name>
  <forward mode='nat'/>
  <bridge name='virbr0' stp='on' delay='0'/>
  <ip address='192.168.122.1' netmask='255.255.255.0'>
    <dhcp>
      <range start='192.168.122.2' end='192.168.122.254'/>
    </dhcp>
  </ip>
</network>
XML
  virsh -c qemu:///system net-define /tmp/default-net.xml
  rm /tmp/default-net.xml
fi
# Solo arrancar si NO está activa (silencia error en cualquier locale)
if ! virsh -c qemu:///system net-list --name 2>/dev/null | grep -qx default; then
  virsh -c qemu:///system net-start default
fi
virsh -c qemu:///system net-autostart default 2>/dev/null || true
ok "network default activa"

# ----------------------------------------------------------------------------
# 4) SSH key del user (gen si no existe)
# ----------------------------------------------------------------------------
step "4/7  SSH key del user $REAL_USER"
if [[ ! -f "$SSH_PUB" ]]; then
  warn "no existe $SSH_PUB — generando..."
  sudo -u "$REAL_USER" ssh-keygen -t ed25519 -N "" -f "$SSH_PRIV" >/dev/null
fi
ok "$SSH_PUB"

# ----------------------------------------------------------------------------
# 5) Base image Ubuntu (cache)
# ----------------------------------------------------------------------------
step "5/7  Base image Ubuntu 24.04 (cache)"
if [[ ! -f "$BASE_IMG" ]]; then
  echo "    descargando ~600 MB..."
  wget -q --show-progress -O "$BASE_IMG" "$BASE_IMG_URL"
fi
ok "$BASE_IMG"

# ----------------------------------------------------------------------------
# 6) Cleanup VM previa + crear nueva
# ----------------------------------------------------------------------------
step "6/7  Creando VM $VM_NAME"
if virsh -c qemu:///system list --all --name | grep -q "^${VM_NAME}$"; then
  warn "VM ya existe, destruyendo primero..."
  virsh -c qemu:///system destroy "$VM_NAME" 2>/dev/null || true
  virsh -c qemu:///system undefine "$VM_NAME" --nvram 2>/dev/null || true
  rm -f "$VM_DISK_PATH" "$CI_ISO"
fi

# Disco con backing image (no copia los 600 MB)
qemu-img create -f qcow2 -F qcow2 -b "$BASE_IMG" "$VM_DISK_PATH" "${VM_DISK}G" >/dev/null
ok "disco: $VM_DISK_PATH (${VM_DISK} GB virtual)"

# Cloud-init: SSH key + paquetes basicos
TMPCI=$(mktemp -d)
PUBKEY=$(cat "$SSH_PUB")
cat > "$TMPCI/user-data" <<USERDATA
#cloud-config
hostname: ${VM_NAME}
manage_etc_hosts: true
users:
  - name: ubuntu
    sudo: ALL=(ALL) NOPASSWD:ALL
    shell: /bin/bash
    ssh_authorized_keys:
      - ${PUBKEY}
package_update: true
package_upgrade: false
packages:
  - git
  - curl
  - make
  - rsync
  - ca-certificates
  - gnupg
ssh_pwauth: false
USERDATA

cat > "$TMPCI/meta-data" <<METADATA
instance-id: ${VM_NAME}
local-hostname: ${VM_NAME}
METADATA

# xorrisofs reemplaza a cloud-localds (no disponible en Arch oficial)
xorrisofs -quiet -output "$CI_ISO" -volid cidata -joliet -rock \
  "$TMPCI/user-data" "$TMPCI/meta-data"
rm -rf "$TMPCI"
ok "cloud-init listo"

virt-install \
  --connect qemu:///system \
  --name "$VM_NAME" \
  --memory "$VM_RAM" \
  --vcpus "$VM_VCPUS" \
  --disk "path=$VM_DISK_PATH,format=qcow2,bus=virtio" \
  --disk "path=$CI_ISO,device=cdrom" \
  --os-variant ubuntu24.04 \
  --network network=default,model=virtio \
  --graphics none \
  --noautoconsole \
  --import \
  --boot uefi >/dev/null
ok "VM definida y arrancando"

# ----------------------------------------------------------------------------
# 7) Esperar IP + SSH
# ----------------------------------------------------------------------------
step "7/7  Esperando IP + SSH ready (cloud-init terminando, hasta 3 min)"
IP=""
for i in {1..60}; do
  IP=$(virsh -c qemu:///system net-dhcp-leases default 2>/dev/null \
        | grep "$VM_NAME" \
        | awk '{print $5}' \
        | cut -d/ -f1 \
        | head -1)
  [[ -n "$IP" ]] && break
  sleep 2
done
[[ -z "$IP" ]] && { fail "VM no obtuvo IP en 120s"; exit 1; }
ok "IP: $IP"

# Wait SSH
for i in {1..60}; do
  if sudo -u "$REAL_USER" ssh -o BatchMode=yes -o StrictHostKeyChecking=no -o ConnectTimeout=3 -i "$SSH_PRIV" "ubuntu@$IP" true 2>/dev/null; then
    ok "SSH responde"
    break
  fi
  sleep 3
done

# ----------------------------------------------------------------------------
# RESUMEN
# ----------------------------------------------------------------------------
cat <<RESUMEN

${GREEN}${BOLD}=================================================${RESET}
${GREEN}${BOLD}  VM ${VM_NAME} lista${RESET}
${GREEN}${BOLD}=================================================${RESET}

  IP:        $IP
  User:      ubuntu  (sudo nopasswd)
  SSH:       ssh -i $SSH_PRIV ubuntu@$IP
  Console:   sudo virsh console $VM_NAME    (Ctrl-] para salir)
  Status:    sudo virsh dominfo $VM_NAME

  Para destruir:
    sudo $0 --destroy

  Repo del proyecto en el host:
    $( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )

RESUMEN
