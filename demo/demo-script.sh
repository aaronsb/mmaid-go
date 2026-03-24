#!/usr/bin/env bash
# Demo script for asciinema recording — tells a story through diagrams
# Called by: asciinema rec --command "bash demo/demo-script.sh"
set -euo pipefail

WIDTH="${COLUMNS:-120}"
MMAID="mmaid -w $WIDTH"
DELAY=3.5

run() {
  local cmd="$1"
  printf '\033[1;32m$\033[0m '
  for (( i=0; i<${#cmd}; i++ )); do
    printf '%s' "${cmd:$i:1}"
    sleep 0.02
  done
  echo
  eval "$cmd"
  sleep "$DELAY"
}

# Run without typing simulation (for piped content)
show() {
  eval "$1"
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

# ═══════════════════════════════════════════════════════
# THE REVEAL — same pie chart, three rendering modes
# ═══════════════════════════════════════════════════════

PIE='pie title Team Allocation
    "Engineering" : 45
    "Design" : 20
    "QA" : 15
    "DevOps" : 12
    "Management" : 8'

header "Braille shading — no theme"
show "echo '$PIE' | $MMAID"

clear
header "Phosphor — green CRT"
show "echo '$PIE' | $MMAID -t phosphor"

clear
header "Full color — monokai"
show "echo '$PIE' | $MMAID -t monokai"

# ═══════════════════════════════════════════════════════
# IDEATE — brainstorm and prioritize
# ═══════════════════════════════════════════════════════

clear
header "Brainstorm — amber"
show "echo 'mindmap
  root((Acme API))
    Auth
      OAuth2
      API Keys
      Rate Limits
    Data
      PostgreSQL
      Redis Cache
      S3 Storage
    Clients
      Web App
      Mobile SDK
      CLI Tool
    Ops
      Monitoring
      Alerts
      Runbooks' | $MMAID -t amber"

clear
header "Prioritize — sunset"
show "echo 'quadrantChart
    title Feature Priority
    x-axis Low Effort --> High Effort
    y-axis Low Impact --> High Impact
    quadrant-1 Do First
    quadrant-2 Plan Carefully
    quadrant-3 Quick Wins
    quadrant-4 Reconsider
    OAuth2: [0.3, 0.9]
    Rate Limits: [0.2, 0.7]
    Mobile SDK: [0.8, 0.8]
    CLI Tool: [0.6, 0.4]
    S3 Storage: [0.4, 0.5]
    Runbooks: [0.7, 0.3]' | $MMAID -t sunset"

# ═══════════════════════════════════════════════════════
# DESIGN — architecture and data model
# ═══════════════════════════════════════════════════════

clear
header "Request flow — blueprint"
show "echo 'graph LR
    A[Client] --> B{API Gateway}
    B -->|Authenticated| C[Service]
    B -->|Rejected| D[401 Error]
    C --> E[(Database)]
    C --> F[(Cache)]
    F -->|miss| E
    E --> G[Response]
    F -->|hit| G' | $MMAID -t blueprint"

clear
header "API sequence — slate"
show "echo 'sequenceDiagram
    participant Client
    participant Gateway
    participant Auth
    participant Service
    participant DB
    Client->>Gateway: POST /orders
    Gateway->>Auth: Validate token
    Auth-->>Gateway: OK
    Gateway->>Service: Create order
    Service->>DB: INSERT
    DB-->>Service: order_id
    Service-->>Gateway: 201 Created
    Gateway-->>Client: {id, status}' | $MMAID -t slate"

clear
header "Order lifecycle — monokai"
show "echo 'stateDiagram-v2
    [*] --> Draft
    Draft --> Submitted : place order
    Submitted --> Processing : payment confirmed
    Processing --> Shipped : dispatch
    Shipped --> Delivered : confirm delivery
    Delivered --> [*]
    Submitted --> Cancelled : cancel
    Processing --> Cancelled : cancel
    Cancelled --> [*]' | $MMAID -t monokai"

clear
header "Data model — gruvbox"
show "echo 'erDiagram
    USER ||--o{ ORDER : places
    ORDER ||--|{ LINE_ITEM : contains
    PRODUCT ||--o{ LINE_ITEM : includes' | $MMAID -t gruvbox"

# ═══════════════════════════════════════════════════════
# BUILD — plan and ship
# ═══════════════════════════════════════════════════════

clear
header "Sprint plan — slate"
show "echo 'gantt
    title Sprint 12
    dateFormat YYYY-MM-DD
    section API
        Auth endpoints     :a1, 2026-03-23, 5d
        Order service      :a2, after a1, 7d
        Rate limiting      :a3, after a1, 4d
    section Frontend
        Login flow         :f1, 2026-03-25, 6d
        Order dashboard    :f2, after f1, 8d
    section QA
        Integration tests  :q1, after a2, 5d
        Load testing       :q2, after q1, 3d' | $MMAID -t slate"

clear
header "Branch strategy — neon"
show "echo 'gitGraph
    commit id: \"init\"
    commit id: \"ci-setup\"
    branch feature/auth
    checkout feature/auth
    commit id: \"oauth2\"
    commit id: \"api-keys\"
    checkout main
    merge feature/auth id: \"merge-auth\" tag: \"v0.1\"
    branch feature/orders
    checkout feature/orders
    commit id: \"create-order\"
    commit id: \"payment\"
    checkout main
    merge feature/orders id: \"merge-orders\" tag: \"v0.2\"
    commit id: \"release\"' | $MMAID -t neon"

clear
header "Task board — monokai"
show "echo 'kanban
  backlog[Backlog]
    t1[S3 integration]
    t2[Mobile SDK]
    t3[Runbooks]
  progress[In Progress]
    t4[Rate limiting]
    t5[Load tests]
  review[Review]
    t6[Order service]
  done[Done]
    t7[Auth endpoints]
    t8[Login flow]
    t9[CI pipeline]' | $MMAID -t monokai"

# ═══════════════════════════════════════════════════════
# MEASURE — metrics and capacity
# ═══════════════════════════════════════════════════════

clear
header "Adoption metrics — gruvbox"
show "echo 'xychart-beta
    title \"API Requests (thousands)\"
    x-axis [Jan, Feb, Mar, Apr, May, Jun]
    y-axis \"Requests\" 0 --> 500
    bar [42, 78, 145, 203, 312, 467]
    line [42, 78, 145, 203, 312, 467]' | $MMAID -t gruvbox"

clear
header "Service footprint — sunset"
show "echo 'treemap-beta
    \"Acme Platform\"
        \"API\"
            \"Auth\": 35
            \"Orders\": 45
            \"Search\": 25
        \"Frontend\"
            \"Web App\": 40
            \"Admin\": 15
        \"Infrastructure\"
            \"Database\": 50
            \"Cache\": 20
            \"Queue\": 15' | $MMAID -t sunset"

# ═══════════════════════════════════════════════════════
# JSON INGEST — real data, no Mermaid syntax needed
# ═══════════════════════════════════════════════════════

clear
header "JSON schema template"
run "$MMAID --json treemap --template"

clear
header "Real system data — one pipe, no jq"
run "lsblk -Jb -o NAME,SIZE,TYPE | $MMAID --json treemap -t blueprint"

clear
header "Instant chart from JSON"
run "echo '{\"Go\":45,\"Rust\":30,\"Python\":25}' | $MMAID --json pie -t phosphor"

# ═══════════════════════════════════════════════════════
sleep 1
printf '\n\033[1;37m  15 diagram types. 11 themes. JSON ingest. One binary.\033[0m\n\n'
sleep 3
