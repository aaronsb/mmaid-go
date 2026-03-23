#!/usr/bin/env bash
# Visual test suite for termaid-go
# Run: ./test_visual.sh
# Review each diagram and check the noted details.

set -euo pipefail

TERMAID="${1:-./termaid}"

if [[ ! -x "$TERMAID" ]]; then
  echo "Building termaid..."
  go build -o "$TERMAID" ./cmd/termaid
fi

pass=0
fail=0

run_test() {
  local name="$1"
  local check="$2"
  local input="$3"

  echo ""
  echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
  echo "TEST: $name"
  echo "CHECK: $check"
  echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
  echo ""
  echo "$input" | "$TERMAID"
  echo ""
}

# ── FLOWCHART TESTS ──────────────────────────────────────────────────────────

run_test "Flowchart: LR basic chain" \
  "Three boxes in a row, connected by arrows (►). Labels centered inside boxes." \
  'graph LR; A[Start] --> B[Process] --> C[End]'

run_test "Flowchart: TD with branching" \
  "Diamond node (◇ markers top/bottom) with two labeled branches. Yes/No labels on edges." \
  'graph TD
    A[Request] --> B{Valid?}
    B -->|Yes| C[Process]
    B -->|No| D[Reject]'

run_test "Flowchart: All node shapes" \
  "Six different shapes: rectangle, rounded (╭╯ corners), diamond (◇), circle (◯), stadium (( ) sides), hexagon (/ \\ corners)." \
  'graph LR
    A[Rectangle] --> B(Rounded) --> C{Diamond}
    C --> D((Circle)) --> E([Stadium]) --> F{{Hexagon}}'

run_test "Flowchart: Edge styles" \
  "Three edge types visible: solid (─►), dotted (┄►), thick (━►). All with arrowheads." \
  'graph LR; A --> B -.-> C ==> D'

run_test "Flowchart: Subgraphs" \
  "Two bordered boxes (Frontend, Backend) each containing nodes. Subgraph labels visible. Edges cross subgraph borders." \
  'graph TB
    subgraph Frontend
        A[Web] --> B[Mobile]
    end
    subgraph Backend
        C[API] --> D[DB]
    end
    A --> C
    B --> C'

run_test "Flowchart: BT (bottom-to-top)" \
  "Arrows point UPWARD (▲). Node C at top, A at bottom. Reversed layout." \
  'graph BT; A --> B --> C'

run_test "Flowchart: RL (right-to-left)" \
  "Arrows point LEFT (◄). Node C on the left, A on the right." \
  'graph RL; A --> B --> C'

run_test "Flowchart: Bidirectional and special arrows" \
  "First edge has arrows on BOTH ends (◄►). Second edge ends with ○. Third ends with ×." \
  'graph LR; A <--> B --o C --x D'

# ── SEQUENCE DIAGRAM TESTS ──────────────────────────────────────────────────

run_test "Sequence: Basic messages" \
  "Two lifelines (Alice, Bob) with solid arrows (►) going right and dotted arrows (◄) returning left. Labels above arrows." \
  'sequenceDiagram
    Alice->>Bob: Hello
    Bob-->>Alice: Hi back'

run_test "Sequence: Multiple participants" \
  "Three lifelines. Messages chain left-to-right then right-to-left. Lifeline chars (┆) visible between messages." \
  'sequenceDiagram
    participant Client
    participant Server
    participant DB
    Client->>Server: Request
    Server->>DB: Query
    DB-->>Server: Result
    Server-->>Client: Response'

run_test "Sequence: Notes and blocks" \
  "A note box (rectangle) appears to the right of Alice. A loop block with [loop] label and dashed section divider encloses messages." \
  'sequenceDiagram
    Alice->>Bob: Start
    Note right of Alice: Important!
    loop Every minute
        Bob->>Alice: Ping
        Alice-->>Bob: Pong
    end'

run_test "Sequence: Actor participant" \
  "First participant shows as a stick figure (O with /|\\ body), not a rectangle box." \
  'sequenceDiagram
    actor User
    participant System
    User->>System: Login
    System-->>User: Token'

# ── CLASS DIAGRAM TESTS ─────────────────────────────────────────────────────

run_test "Class diagram: Inheritance" \
  "Animal box at top with members listed. Duck and Fish boxes below connected by lines with triangle markers (▽/△ for inheritance)." \
  'classDiagram
    Animal <|-- Duck
    Animal <|-- Fish
    Animal : +int age
    Animal : +makeSound()
    Duck : +swim()
    Fish : +int size'

run_test "Class diagram: Interface annotation" \
  "Shape box should show «interface» annotation above the class name. Divider line (├─┤) separates name from members." \
  'classDiagram
    class Shape {
        <<interface>>
        +area() float
        +perimeter() float
    }'

# ── ER DIAGRAM TESTS ────────────────────────────────────────────────────────

run_test "ER diagram: Basic relationships" \
  "Two entity boxes (CUSTOMER, ORDER) with attributes listed. Connecting line with cardinality text (1, 0..*, etc.) near endpoints. Label 'places' on the relationship line." \
  'erDiagram
    CUSTOMER ||--o{ ORDER : places
    CUSTOMER {
        int id PK
        string name
    }
    ORDER {
        int id PK
        date created
    }'

# ── STATE DIAGRAM TESTS ─────────────────────────────────────────────────────

run_test "State diagram: Basic transitions" \
  "Start marker (●) at top, end marker (◉) at bottom. Rounded state boxes (╭╯ corners). Arrows between states." \
  'stateDiagram-v2
    [*] --> Idle
    Idle --> Processing : start
    Processing --> Done
    Done --> [*]'

# ── PIE CHART TESTS ─────────────────────────────────────────────────────────

run_test "Pie chart: Basic" \
  "Horizontal bar chart with right-aligned labels. Different fill patterns (█▓░▒). Percentages shown. Bars proportional to values." \
  'pie title Browser Market Share
    "Chrome" : 65
    "Firefox" : 15
    "Safari" : 10
    "Edge" : 8
    "Other" : 2'

# ── GIT GRAPH TESTS ─────────────────────────────────────────────────────────

run_test "Git graph: Branch and merge (LR)" \
  "Two horizontal branch lines (main, develop). Commit dots (●) along each. Fork point has ┬, merge point has ┴. Branch labels on the left. Commit IDs below dots." \
  'gitGraph
    commit id: "A"
    commit id: "B"
    branch develop
    checkout develop
    commit id: "C"
    commit id: "D"
    checkout main
    merge develop id: "E"
    commit id: "F"'

run_test "Git graph: Tags" \
  "Tag text in [brackets] appears ABOVE the tagged commit dot. Commit ID below." \
  'gitGraph
    commit id: "init"
    commit id: "feat" tag: "v1.0"
    commit id: "fix"'

run_test "Git graph: TB orientation" \
  "Branches run VERTICALLY (top to bottom). Fork/merge lines are HORIZONTAL with ├/┤ junctions. Branch names at top." \
  'gitGraph TB:
    commit id: "A"
    branch dev
    checkout dev
    commit id: "B"
    checkout main
    merge dev id: "C"'

# ── BLOCK DIAGRAM TESTS ─────────────────────────────────────────────────────

run_test "Block diagram: Grid layout" \
  "Three boxes in a row (3-column grid). Arrows connecting them left-to-right." \
  'block-beta
    columns 3
    A["Input"] B["Transform"] C["Output"]
    A-->B
    B-->C'

# ── TREEMAP TESTS ────────────────────────────────────────────────────────────

run_test "Treemap: Nested sections" \
  "Two section boxes with DASHED borders (┄). Each contains leaf boxes with SOLID borders. Leaf boxes show label and numeric value. Wider boxes for larger values." \
  'treemap-beta
    "Code"
        "Go": 50
        "Python": 30
    "Docs"
        "README": 10
        "API": 10'

# ── ASCII MODE TEST ──────────────────────────────────────────────────────────

run_test "ASCII mode: Flowchart" \
  "All box-drawing uses +, -, | characters. Arrows use > < v ^. No Unicode characters anywhere." \
  'graph LR; A[Start] --> B{Check} --> C[Done]'

# Re-run the last one in ASCII
echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "TEST: ASCII mode: Flowchart (--ascii flag)"
echo "CHECK: All box-drawing uses +, -, | characters. Arrows use > < v ^. No Unicode characters anywhere."
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""
echo 'graph LR; A[Start] --> B{Check} --> C[Done]' | "$TERMAID" --ascii
echo ""

echo ""
echo "════════════════════════════════════════════════════════════════════════"
echo "Visual test suite complete. Review each diagram above."
echo "════════════════════════════════════════════════════════════════════════"
