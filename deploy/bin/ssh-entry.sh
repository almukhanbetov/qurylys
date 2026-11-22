#!/usr/bin/env bash
# Forced command for the GitHub Actions deploy key (see docs/deployment.md):
#   restrict,command="/opt/qurylys/bin/ssh-entry.sh" ssh-ed25519 AAAA... qurylys-gha-deploy
# The key can run only these two commands; anything else is rejected.
# Arguments are validated again by deploy.sh against the allowed image repositories.
set -euo pipefail

read -r -a args <<< "${SSH_ORIGINAL_COMMAND:-}"

case "${args[0]:-}" in
  deploy)
    [[ ${#args[@]} -eq 3 ]] || { echo "usage: deploy <backend-ref> <frontend-ref>" >&2; exit 2; }
    exec /opt/qurylys/bin/deploy.sh deploy "${args[1]}" "${args[2]}"
    ;;
  status)
    [[ ${#args[@]} -eq 1 ]] || { echo "usage: status" >&2; exit 2; }
    exec /opt/qurylys/bin/deploy.sh status
    ;;
  *)
    echo "allowed commands: deploy <backend-ref> <frontend-ref> | status" >&2
    exit 2
    ;;
esac
