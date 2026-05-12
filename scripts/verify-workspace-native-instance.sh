#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'EOF'
Usage:
  scripts/verify-workspace-native-instance.sh status <profile-id> [launch-port]
  scripts/verify-workspace-native-instance.sh multi-shop <profile-id-1> <profile-id-2> [launch-port]
  scripts/verify-workspace-native-instance.sh reclaim <profile-id> [launch-port]

Examples:
  scripts/verify-workspace-native-instance.sh status alibaba:b2b-222082061706256a1a
  scripts/verify-workspace-native-instance.sh multi-shop alibaba:shop-a alibaba:shop-b
  scripts/verify-workspace-native-instance.sh reclaim alibaba:shop-a
EOF
}

CMD="${1:-}"

if [[ "${CMD}" == "-h" || "${CMD}" == "--help" || "${CMD}" == "help" ]]; then
  usage
  exit 0
fi

if [[ $# -lt 2 ]]; then
  usage
  exit 1
fi

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
DATA_DIR="${ROOT_DIR}/data/managed-profiles"
shift

print_header() {
  printf '\n== %s ==\n' "$1"
}

profile_dir_name() {
  printf '%s' "$1" | sed 's/:/__/g'
}

show_processes() {
  local pattern="$1"
  pgrep -fal "$pattern" || true
}

show_targets() {
  local port="$1"
  curl -fsS "http://127.0.0.1:${port}/json/list" || true
}

status_cmd() {
  local profile_id="$1"
  local launch_port="${2:-19876}"
  local dir_name
  dir_name="$(profile_dir_name "$profile_id")"

  print_header "Process"
  show_processes "${dir_name}|remote-debugging-port="

  print_header "Managed Profile Dir"
  local path="${DATA_DIR}/${dir_name}"
  if [[ -d "${path}" ]]; then
    printf 'present: %s\n' "${path}"
  else
    printf 'missing: %s\n' "${path}"
  fi

  print_header "LaunchServer Targets"
  show_targets "${launch_port}"
}

multi_shop_cmd() {
  local profile_a="$1"
  local profile_b="$2"
  local launch_port="${3:-19876}"

  status_cmd "${profile_a}" "${launch_port}"
  status_cmd "${profile_b}" "${launch_port}"
}

reclaim_cmd() {
  local profile_id="$1"
  local launch_port="${2:-19876}"
  local dir_name
  dir_name="$(profile_dir_name "$profile_id")"

  print_header "Reclaim Process Check"
  show_processes "${dir_name}|remote-debugging-port="

  print_header "Reclaim Dir Check"
  local path="${DATA_DIR}/${dir_name}"
  if [[ -d "${path}" ]]; then
    printf 'still present: %s\n' "${path}"
  else
    printf 'removed: %s\n' "${path}"
  fi

  print_header "LaunchServer Targets"
  show_targets "${launch_port}"
}

case "${CMD}" in
  status)
    if [[ $# -lt 1 ]]; then
      usage
      exit 1
    fi
    status_cmd "$@"
    ;;
  multi-shop)
    if [[ $# -lt 2 ]]; then
      usage
      exit 1
    fi
    multi_shop_cmd "$@"
    ;;
  reclaim)
    if [[ $# -lt 1 ]]; then
      usage
      exit 1
    fi
    reclaim_cmd "$@"
    ;;
  *)
    printf '[ERROR] unknown command: %s\n' "${CMD}" >&2
    usage
    exit 1
    ;;
esac
