#!/usr/bin/env bash
# Installs Ubuntu packages on a GitHub Actions runner, riding out mirror
# failures: apt retries each download, and the whole update and install is
# retried with a growing pause, because a mirror outage can outlast apt's
# own retries.
#
# usage: .github/scripts/apt-install.sh PACKAGE...
set -euo pipefail

attempts=4
for attempt in $(seq 1 "$attempts"); do
  if sudo apt-get -o Acquire::Retries=5 update &&
    sudo apt-get -o Acquire::Retries=5 install -y "$@"; then
    exit 0
  fi
  if [ "$attempt" -lt "$attempts" ]; then
    delay=$((attempt * 30))
    echo "::warning::apt failed on attempt $attempt of $attempts; retrying in ${delay}s"
    sleep "$delay"
  fi
done
echo "::error::apt could not install: $*"
exit 1
