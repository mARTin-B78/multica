#!/bin/sh
set -eu

# Portainer's ordinary name/value grid is one line and therefore unsuitable for
# a PEM. This wrapper receives the key as a read-only host bind mount, validates
# only its shape, and exports it for the official Multica entrypoint.

key_file=/run/secrets/multica_github_app_private_key

if [ ! -s "$key_file" ]; then
  echo "GitHub App private-key file is missing or empty." >&2
  exit 78
fi

key=$(cat "$key_file")
line_count=$(printf '%s\n' "$key" | wc -l | tr -d ' ')
case "$key" in
  *"-----BEGIN "*"PRIVATE KEY-----"*"-----END "*"PRIVATE KEY-----"*) ;;
  *)
    echo "GitHub App private-key file is not PEM-formatted." >&2
    exit 78
    ;;
esac
if [ "$line_count" -lt 3 ]; then
  echo "GitHub App private-key file must preserve PEM line breaks." >&2
  exit 78
fi

export GITHUB_APP_PRIVATE_KEY="$key"
key=""
exec /app/entrypoint.sh "$@"
