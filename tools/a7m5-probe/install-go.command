#!/bin/zsh
set -u

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
cd "$SCRIPT_DIR" || exit 1
bash "$SCRIPT_DIR/install-go.sh"
status=$?

echo
if [[ $status -ne 0 ]]; then
    echo "Installation or build failed with exit code $status."
else
    echo "Installation and build completed."
fi
echo "Press any key to close this window."
read -k 1
exit $status
