#!/usr/bin/env bash
# Record an asciinema demo of mmaid diagram types
# Usage: ./demo/record.sh
#   Runs: asciinema rec demo/mmaid-demo.cast --command "bash demo/demo-script.sh"
#   then tools/castwidth.py, which fails when any line is wider than the
#   recorded terminal, and agg for the gif.

set -euo pipefail
cd "$(dirname "$0")/.."

CAST_FILE="demo/mmaid-demo.cast"
SCRIPT="demo/demo-script.sh"

go build -o mmaid ./cmd/mmaid
export PATH="$PWD:$PATH"

echo "Recording to $CAST_FILE ..."
asciinema rec "$CAST_FILE" \
  --window-size 120x80 \
  --command "bash $SCRIPT" \
  --overwrite

echo ""
echo "Checking that no line is wider than the recorded terminal ..."
python3 tools/castwidth.py "$CAST_FILE"

echo "Rendering demo/mmaid-demo.gif ..."
agg "$CAST_FILE" demo/mmaid-demo.gif

echo ""
echo "Done! Cast saved to $CAST_FILE"
echo "Preview:  asciinema play $CAST_FILE"
echo "Upload:   asciinema upload $CAST_FILE"
