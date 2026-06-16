#!/usr/bin/env bash
# test-vps-libvirt.sh — crea una VM Ubuntu real con libvirt/KVM y corre el
# install-vps.sh adentro. ~99% fiel a un VPS de verdad (kernel propio,
# networking real, systemd real, sin DinD tricks).
#
# Uso (TODOS requieren root o membresía del grupo libvirt):
#   sudo ./test-vps-libvirt.sh check       # verifica deps + SSH key
#   sudo ./test-vps-libvirt.sh up          # crea VM + cloud-init (Ubuntu 24.04)
#   sudo ./test-vps-libvirt.sh install     # clona repo + corre install-vps.sh
#   sudo ./test-vps-libvirt.sh smoke       # curls contra http://<ip-vm>/
#   sudo ./test-vps-libvirt.sh ssh         # entra a la VM
#   sudo ./test-vps-libvirt.sh status      # virsh dominfo + IP
#   sudo ./test-vps-libvirt.sh logs        # tail journalctl + docker logs
#   sudo ./test-vps-libvirt.sh down        # destroy + undefine + clean
#
# Requisitos (Arch):
#   pacman -S libvirt qemu-base virt-install dnsmasq bridge-utils \
#             cloud-image-utils edk2-ovmf
#   systemctl enable --now libvirtd
#   usermod -aG libvirt $USER  # logout/login
#   (o usar sudo siempre)

set -euo pipefail

# ----------------------------------------------------------------------------
# Config
# ----------------------------------------------------------------------------
VM_NAME="${VM_NAME:-domain-vps-test}"
VM_RAM_MB="${VM_RAM_MB:-4096}"
VM_VCPUS="${VM_VCPUS:-2}"
VM_DISK_GB="${VM_DISK_GB:-15}"
UBUNTU_VERSION="24.04"
UBUNTU_CODENAME="noble"

IMG_DIR="/var/lib/libvirt/images"
BASE_IMG="$IMG_DIR/ubuntu-${UBUNTU_VERSION}-cloud.qcow2"
BASE_IMG_URL="https://cloud-images.ubuntu.com/${UBUNTU_CODENAME}/current/${UBUNTU_CODENAME}-server-cloudimg-amd64.img"
VM_DISK_PATH="$IMG_DIR/${VM_NAME}.qcow2"
CLOUD_INIT_ISO="$IMG_DIR/${VM_NAME}-cloudinit.iso"

# Detectar SSH key del user real (no de root cuando se llama via sudo)
REAL_USER="${SUDO_USER:-${USER}}"
REAL_HOME=$(getent passwd "$REAL_USER" | cut -d: -f6)
SSH_KEY_PUB="${SSH_KEY_PUB:-$REAL_HOME/.ssh/id_ed25519.pub}"
SSH_KEY_PRIV="${SSH_KEY_PRIV:-$REAL_HOME/.ssh/id_ed25519}"

REPO_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
LOG_DIR="${LOG_DIR:-/tmp/${VM_NAME}-logs}"
mkdir -p "$LOG_DIR"

# ----------------------------------------------------------------------------
# Estilo
# ----------------------------------------------------------------------------
BOLD=$'\033[1m'; RESET=$'\033[0m'
GREEN=$'\033[32m'; YELLOW=$'\033[33m'; RED=$'\033[31m'; DIM=$'\033[2m'
step() { echo ""; echo "${BOLD}==> $1${RESET}"; }
ok()   { echo "${GREEN}    ✓${RESET} $1"; }
warn() { echo "${YELLOW}    !${RESET} $1"; }
fail() { echo "${RED}    ✗${RESET} $1" >&2; }
info() { echo "${DIM}    ·${RESET} $1"; }

# ----------------------------------------------------------------------------
# Helpers
# ----------------------------------------------------------------------------
need_root() {
  if [[ $EUID -ne 0 ]]; then
    fail "Este script necesita root (libvirt system). Corré con sudo."
    exit 1
  fi
}

require_cmd() {
  command -v "$1" >/dev/null 2>&1 || { fail "comando requerido no encontrado: $1"; return 1; }
}

vm_ip() {
  virsh -c qemu:///system net-dhcp-leases default 2>/dev/null \
    | grep "$VM_NAME" \
    | awk '{print $5}' \
    | cut -d/ -f1 \
    | head -1
}

wait_for_ip() {
  local tries=60
  local ip=""
  while [[ $tries -gt 0 ]]; do
    ip=$(vm_ip)
    [[ -n "$ip" ]] && { echo "$ip"; return 0; }
    sleep 2
    tries=$((tries-1))
  done
  return 1
}

wait_for_ssh() {
  local ip="$1"
  local tries=60
  while [[ $tries -gt 0 ]]; do
    if su - "$REAL_USER" -c "ssh -o BatchMode=yes -o StrictHostKeyChecking=no -o ConnectTimeout=3 -i $SSH_KEY_PRIV ubuntu@$ip 'true'" 2>/dev/null; then
      return 0
    fi
    sleep 3
    tries=$((tries-1))
  done
  return 1
}

ssh_vm() {
  local ip="$1"; shift
  su - "$REAL_USER" -c "ssh -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -i $SSH_KEY_PRIV ubuntu@$ip $*"
}

scp_to_vm() {
  local ip="$1"; local src="$2"; local dst="$3"
  su - "$REAL_USER" -c "scp -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -i $SSH_KEY_PRIV $src ubuntu@$ip:$dst"
}

# ----------------------------------------------------------------------------
# Comandos
# ----------------------------------------------------------------------------
cmd_check() {
  step "Verificando dependencias"
  local all_ok=1
  for c in virsh virt-install qemu-img wget cloud-localds; do
    if require_cmd "$c"; then ok "$c"; else all_ok=0; fi
  done
  [[ $all_ok -eq 0 ]] && {
    echo ""
    echo "Instalá lo que falta (Arch):"
    echo "  sudo pacman -S libvirt qemu-base virt-install dnsmasq bridge-utils cloud-image-utils edk2-ovmf"
    echo "  sudo systemctl enable --now libvirtd"
    exit 1
  }

  step "Verificando libvirtd"
  systemctl is-active --quiet libvirtd && ok "libvirtd corriendo" || { fail "libvirtd no corre. systemctl start libvirtd"; exit 1; }

  step "Verificando default network de libvirt"
  if virsh -c qemu:///system net-info default >/dev/null 2>&1; then
    if virsh -c qemu:///system net-info default | grep -q "Active:.*yes"; then
      ok "network default activa"
    else
      warn "network default existe pero no activa, arrancando..."
      virsh -c qemu:///system net-start default || { fail "no pude arrancar network default"; exit 1; }
      ok "network default activa"
    fi
  else
    fail "network default no existe — corré: virsh net-define /usr/share/libvirt/networks/default.xml && virsh net-start default && virsh net-autostart default"
    exit 1
  fi

  step "Verificando SSH key del user"
  if [[ ! -f "$SSH_KEY_PUB" ]]; then
    warn "no existe $SSH_KEY_PUB"
    echo "Generala con: ssh-keygen -t ed25519 -f $SSH_KEY_PRIV"
    exit 1
  fi
  ok "SSH key: $SSH_KEY_PUB"

  step "Verificando KVM"
  [[ -e /dev/kvm ]] && ok "/dev/kvm disponible (aceleración HW)" || warn "/dev/kvm no disponible — la VM va a ser MUY lenta (QEMU emulation)"

  echo ""
  ok "Todo OK. Podés correr: sudo $0 up"
}

cmd_up() {
  need_root

  if virsh -c qemu:///system list --all --name | grep -q "^${VM_NAME}$"; then
    fail "VM '$VM_NAME' ya existe. Usá: sudo $0 down  primero."
    exit 1
  fi

  step "Descargando cloud image Ubuntu ${UBUNTU_VERSION} (si no existe)"
  if [[ ! -f "$BASE_IMG" ]]; then
    wget -q --show-progress -O "$BASE_IMG" "$BASE_IMG_URL"
    ok "imagen base: $BASE_IMG"
  else
    ok "imagen base ya presente"
  fi

  step "Creando disco de la VM (qcow2 con backing image)"
  qemu-img create -f qcow2 -F qcow2 -b "$BASE_IMG" "$VM_DISK_PATH" "${VM_DISK_GB}G"
  ok "disco: $VM_DISK_PATH (${VM_DISK_GB} GB virtual, backing ${BASE_IMG})"

  step "Generando cloud-init"
  local ssh_pubkey
  ssh_pubkey=$(cat "$SSH_KEY_PUB")
  local tmpdir
  tmpdir=$(mktemp -d)

  cat > "$tmpdir/user-data" <<USERDATA
#cloud-config
hostname: ${VM_NAME}
manage_etc_hosts: true
users:
  - name: ubuntu
    sudo: ALL=(ALL) NOPASSWD:ALL
    shell: /bin/bash
    ssh_authorized_keys:
      - ${ssh_pubkey}
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

  cat > "$tmpdir/meta-data" <<METADATA
instance-id: ${VM_NAME}
local-hostname: ${VM_NAME}
METADATA

  cloud-localds "$CLOUD_INIT_ISO" "$tmpdir/user-data" "$tmpdir/meta-data"
  rm -rf "$tmpdir"
  ok "cloud-init ISO: $CLOUD_INIT_ISO"

  step "Creando VM (virt-install)"
  virt-install \
    --connect qemu:///system \
    --name "$VM_NAME" \
    --memory "$VM_RAM_MB" \
    --vcpus "$VM_VCPUS" \
    --disk "path=$VM_DISK_PATH,format=qcow2,bus=virtio" \
    --disk "path=$CLOUD_INIT_ISO,device=cdrom" \
    --os-variant "ubuntu24.04" \
    --network "network=default,model=virtio" \
    --graphics none \
    --noautoconsole \
    --import \
    --boot uefi
  ok "VM creada"

  step "Esperando IP de la VM (DHCP)"
  local ip
  ip=$(wait_for_ip) || { fail "VM no obtuvo IP en 120s"; exit 1; }
  ok "IP: $ip"

  step "Esperando SSH ready (cloud-init terminando)"
  if wait_for_ssh "$ip"; then
    ok "SSH OK"
  else
    fail "SSH no responde en 180s. La VM existe pero cloud-init quizá falló."
    info "Inspeccioná con: sudo virsh console $VM_NAME  (Ctrl-] para salir)"
    exit 1
  fi

  cat <<HINT

${BOLD}VM ${VM_NAME} arriba en $ip${RESET}

Próximos pasos:
  sudo $0 install        # clona repo + corre install-vps.sh + colecta logs
  sudo $0 ssh            # entra a la VM
  sudo $0 smoke          # curls contra http://$ip/
  sudo $0 down           # destruye todo

Acceso manual:
  ssh -i $SSH_KEY_PRIV ubuntu@$ip

HINT
}

cmd_install() {
  need_root
  local ip
  ip=$(vm_ip) || { fail "no encontré IP de $VM_NAME (¿está arriba?)"; exit 1; }
  [[ -z "$ip" ]] && { fail "VM sin IP"; exit 1; }
  ok "VM en $ip"

  step "Copiando repo a la VM"
  ssh_vm "$ip" "rm -rf /tmp/domain && mkdir -p /tmp/domain" >/dev/null
  su - "$REAL_USER" -c "rsync -az --exclude=.git --exclude=.claude --exclude=services/.env --exclude=test-vps-local.sh --exclude=test-vps-libvirt.sh -e 'ssh -o StrictHostKeyChecking=no -i $SSH_KEY_PRIV' $REPO_DIR/ ubuntu@$ip:/tmp/domain/"
  ok "repo en /tmp/domain de la VM"

  step "Pre-llenando .env con passwords aleatorias"
  ssh_vm "$ip" "bash -c '
    cp /tmp/domain/services/.env.example /tmp/domain/services/.env
    chmod 600 /tmp/domain/services/.env
    for v in POSTGRES_PASSWORD APP_USER_PASSWORD APP_ADMIN_PASSWORD MINIO_ROOT_PASSWORD BACKUP_GPG_PASSPHRASE; do
      pass=\$(openssl rand -base64 48 | tr -d /+= | head -c 32)
      sed -i \"s|^\$v=CHANGE_ME|\$v=\$pass|\" /tmp/domain/services/.env
    done
  '"
  ok ".env listo"

  step "Corriendo install-vps.sh adentro (~5-20 min)"
  ssh_vm "$ip" "cd /tmp/domain && sudo bash ./services/install-vps.sh" 2>&1 | tee "$LOG_DIR/01-install-vps.log"
  local s1=${PIPESTATUS[0]}
  ok "install-vps.sh exit=$s1"

  step "make up (si install no lo hizo solo)"
  ssh_vm "$ip" "cd /opt/services && sudo make up 2>&1 || true" | tee "$LOG_DIR/02-make-up.log" >/dev/null
  sleep 5

  step "Colectando logs de los containers"
  for svc in domain-postgres domain-minio domain-minio-bootstrap domain-migrate domain-backend domain-frontend domain-caddy; do
    ssh_vm "$ip" "sudo docker logs $svc 2>&1" > "$LOG_DIR/03-logs-${svc}.log" 2>&1 || true
  done
  ok "logs en $LOG_DIR"

  ssh_vm "$ip" "sudo docker ps --format 'table {{.Names}}\t{{.Status}}'" | tee "$LOG_DIR/04-ps.log"
}

cmd_smoke() {
  need_root
  local ip
  ip=$(vm_ip) || { fail "VM no encontrada"; exit 1; }
  step "Smoke tests contra http://$ip/"
  {
    echo "--- GET / ---"
    curl -sS -o /dev/null -w "HTTP %{http_code} | %{size_download} bytes | %{time_total}s\n" "http://$ip/" 2>&1
    echo "--- GET /healthz ---"
    curl -sS -o /dev/null -w "HTTP %{http_code}\n" "http://$ip/healthz" 2>&1
    echo "--- GET /api/v1/orgs (sin auth → esperado 401) ---"
    curl -sS -o /dev/null -w "HTTP %{http_code}\n" "http://$ip/api/v1/orgs" 2>&1
    echo "--- POST /mcp (sin auth → esperado 401) ---"
    curl -sS -o /dev/null -w "HTTP %{http_code}\n" -X POST "http://$ip/mcp" 2>&1
  } | tee "$LOG_DIR/05-smoke.log"
}

cmd_ssh() {
  need_root
  local ip
  ip=$(vm_ip) || { fail "VM no encontrada"; exit 1; }
  exec su - "$REAL_USER" -c "ssh -i $SSH_KEY_PRIV ubuntu@$ip"
}

cmd_status() {
  need_root
  virsh -c qemu:///system list --all | grep -E "Id|---|$VM_NAME" || true
  local ip
  ip=$(vm_ip || true)
  [[ -n "${ip:-}" ]] && info "IP: $ip"
}

cmd_logs() {
  need_root
  ls -la "$LOG_DIR"
  echo ""
  echo "Para ver uno: less $LOG_DIR/01-install-vps.log"
}

cmd_down() {
  need_root
  step "Destruyendo VM $VM_NAME"
  virsh -c qemu:///system destroy "$VM_NAME" 2>/dev/null || true
  virsh -c qemu:///system undefine "$VM_NAME" --nvram 2>/dev/null || true
  rm -f "$VM_DISK_PATH" "$CLOUD_INIT_ISO"
  ok "VM destruida + disco + cloud-init iso removidos"
  info "Logs preservados en $LOG_DIR (eliminar con: rm -rf $LOG_DIR)"
  info "Base image preservada en $BASE_IMG (para reuse rápido)"
}

# ----------------------------------------------------------------------------
# Main
# ----------------------------------------------------------------------------
case "${1:-}" in
  check)   cmd_check ;;
  up)      cmd_up ;;
  install) cmd_install ;;
  smoke)   cmd_smoke ;;
  ssh)     cmd_ssh ;;
  status)  cmd_status ;;
  logs)    cmd_logs ;;
  down)    cmd_down ;;
  ""|help|-h|--help) sed -n '2,22p' "$0" ;;
  *) fail "comando desconocido: $1"; exit 2 ;;
esac
