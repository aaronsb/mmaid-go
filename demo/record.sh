#!/usr/bin/env bash
# Record an asciinema demo of mmaid diagram types
# Usage: ./demo/record.sh
#   Runs: asciinema rec demo/mmaid-demo.cast --command "bash demo/demo-script.sh"

set -euo pipefail
cd "$(dirname "$0")/.."

CAST_FILE="demo/mmaid-demo.cast"
SCRIPT="demo/demo-script.sh"

echo "Recording to $CAST_FILE ..."
asciinema rec "$CAST_FILE" \
  --window-size 120x80 \
  --command "bash $SCRIPT" \
  --overwrite

echo ""
echo "Done! Cast saved to $CAST_FILE"
echo "Preview:  asciinema play $CAST_FILE"
echo "Upload:   asciinema upload $CAST_FILE"
