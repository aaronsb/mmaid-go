#!/usr/bin/env bash
# Demo script for asciinema recording — showcases all mmaid diagram types
# Called by: asciinema rec --command "bash demo/demo-script.sh"
set -euo pipefail

WIDTH="${COLUMNS:-120}"  # match asciinema --cols or current terminal
MMAID="mmaid -w $WIDTH"
DELAY=3  # seconds between diagrams

# Simulates typing a command then runs it
run() {
  local cmd="$1"
  # Print prompt + command char by char
  printf '\033[1;32m$\033[0m '
  for (( i=0; i<${#cmd}; i++ )); do
    printf '%s' "${cmd:$i:1}"
    sleep 0.02
  done
  echo
  eval "$cmd"
  sleep "$DELAY"
}

header() {
  printf '\n\033[1;36m── %s ──\033[0m\n\n' "$1"
  sleep 0.5
}

clear
printf '\033[1;37m'
cat <<'BANNER'

  ┌─────────────────────────────────────────────┐
  │  mmaid — Mermaid diagrams as terminal art   │
  │  https://github.com/aaronsb/mmaid-go        │
  └─────────────────────────────────────────────┘

BANNER
printf '\033[0m'
sleep 2

# --- Structural diagrams (solid-background themes) ---

header "Flowchart — blueprint theme"
run "$MMAID --demo flowchart -t blueprint"

clear
header "Sequence Diagram — slate theme"
run "$MMAID --demo sequence -t slate"

clear
header "Class Diagram — blueprint theme"
run "$MMAID --demo class -t blueprint"

clear
header "ER Diagram — slate theme"
run "$MMAID --demo er -t slate"

clear
header "State Diagram — monokai theme"
run "$MMAID --demo state -t monokai"

clear
header "Git Graph — gruvbox theme"
run "$MMAID --demo gitgraph -t gruvbox"

clear
header "Block Diagram — sunset theme"
run "$MMAID --demo block -t sunset"

# --- Charts & data ---

clear
header "Pie Chart — monokai theme"
run "$MMAID --demo pie -t monokai"

clear
header "XY Chart — gruvbox theme"
run "$MMAID --demo xychart -t gruvbox"

clear
header "Treemap — sunset theme"
run "$MMAID --demo treemap -t sunset"

clear
header "Quadrant Chart — blueprint theme"
run "$MMAID --demo quadrant -t blueprint"

# --- Planning & organization ---

clear
header "Gantt Chart — slate theme"
run "$MMAID --demo gantt -t slate"

clear
header "Timeline — gruvbox theme"
run "$MMAID --demo timeline -t gruvbox"

clear
header "Kanban Board — monokai theme"
run "$MMAID --demo kanban -t monokai"

clear
header "Mindmap — sunset theme"
run "$MMAID --demo mindmap -t sunset"

# --- Pipe demo with real system data ---

clear
header "Pipe it — lsblk JSON → jq → mmaid treemap"
# Show the command readably (typing simulation), then run from a temp script
PIPE_DISPLAY="lsblk -Jb -o NAME,SIZE,TYPE | jq -r '...' | mmaid -w $WIDTH -t blueprint"
printf '\033[1;32m$\033[0m '
for (( i=0; i<${#PIPE_DISPLAY}; i++ )); do
  printf '%s' "${PIPE_DISPLAY:$i:1}"
  sleep 0.02
done
echo
# Now run the real pipeline
lsblk -Jb -o NAME,SIZE,TYPE | jq -r '
  def human: if . >= 1073741824 then
    (((. / 1073741824 * 10 | floor) / 10) | tostring) + "G"
  else ((. / 1048576 | floor) | tostring) + "M" end;
  def weight: [(. / 1073741824 | floor), 1] | max;
  "treemap-beta", "  \"Storage\"",
  (.blockdevices[] | select(.type == "disk") |
    "    \"" + .name + "\"",
    (.children // [] | .[] |
      "      \"" + .name + " (" + (.size | human) + ")\":" + (.size | weight | tostring)))
' | $MMAID -t blueprint
sleep "$DELAY"

# --- Closing ---
sleep 1
printf '\n\033[1;37m  15 diagram types. 11 themes. One binary. No dependencies.\033[0m\n\n'
sleep 3
