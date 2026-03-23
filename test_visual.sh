#!/usr/bin/env bash
# Visual test suite for termaid-go
# Run: ./test_visual.sh [theme]
# Examples:
#   ./test_visual.sh              # no theme (plain)
#   ./test_visual.sh blueprint    # blueprint theme (solid colored regions)
#   ./test_visual.sh slate        # slate theme (gray tones)
#   ./test_visual.sh default      # default theme (colored text, no backgrounds)

set -euo pipefail

TERMAID="${TERMAID:-./termaid}"
THEME="${1:-}"

# Always rebuild to pick up latest changes
echo "Building termaid..."
go build -o "$TERMAID" ./cmd/termaid

THEME_FLAG=""
THEME_LABEL="(no theme)"
if [[ -n "$THEME" ]]; then
  THEME_FLAG="--theme $THEME"
  THEME_LABEL="theme: $THEME"
fi

pass=0
total=0

run_test() {
  local name="$1"
  local check="$2"
  local input="$3"

  total=$((total + 1))
  echo ""
  echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
  echo "TEST $total: $name [$THEME_LABEL]"
  echo "CHECK: $check"
  echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
  echo ""
  echo "$input" | $TERMAID $THEME_FLAG
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
  "A note box appears to the right of Alice. A loop block with [loop] label and dashed divider encloses messages." \
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
  "Two entity boxes (CUSTOMER, ORDER) with attributes listed. Connecting line with cardinality. Label 'places' on the relationship line." \
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

run_test "Pie chart: Circular" \
  "Circular pie with colored wedges (or braille patterns without theme). Legend on the right with color swatches and percentages." \
  'pie title Browser Market Share
    "Chrome" : 65
    "Firefox" : 15
    "Safari" : 10
    "Edge" : 8
    "Other" : 2'

run_test "Pie chart: showData" \
  "Circular pie with both percentages and raw values in the legend." \
  'pie showData
    title Project Hours
    "Development" : 45
    "Testing" : 20
    "Design" : 15
    "Management" : 10
    "Ops" : 7
    "Docs" : 3'

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
  "Two section boxes with DASHED borders (┄). Each contains leaf boxes with SOLID borders. With theme: per-section hue + depth shading." \
  'treemap-beta
    "Code"
        "Go": 50
        "Python": 30
    "Docs"
        "README": 10
        "API": 10'

# ── TIMELINE TESTS ──────────────────────────────────────────────────────────

run_test "Timeline: Social media history" \
  "Horizontal axis with dots (●). Period labels below axis. Event boxes stacked above each dot." \
  'timeline
    title History of Social Media
    2002 : LinkedIn
    2004 : Facebook : Google
    2005 : YouTube
    2006 : Twitter'

# ── KANBAN TESTS ─────────────────────────────────────────────────────────────

run_test "Kanban: Task board" \
  "Three columns (Todo, In Progress, Done) with cards inside. With theme: each column has distinct hue, cards lighter shade." \
  'kanban
  col1[Todo]
    t1[Design API]
    t2[Write tests]
  col2[In Progress]
    t3[Implement parser]
  col3[Done]
    t4[Setup CI]
    t5[Deploy v1]'

# ── MINDMAP TESTS ────────────────────────────────────────────────────────────

run_test "Mindmap: Project structure" \
  "Root node on left, children branching right. Horizontal and vertical edge connectors. Boxes around each node." \
  'mindmap
  root((Project))
    Frontend
      React
      CSS
    Backend
      Go
      PostgreSQL
    DevOps
      Docker
      K8s'

# ── QUADRANT CHART TESTS ────────────────────────────────────────────────────

run_test "Quadrant: Campaign analysis" \
  "2×2 grid with dashed center lines. Quadrant labels in each section. Points plotted with ● and labeled." \
  'quadrantChart
    title Reach and engagement
    x-axis Low Reach --> High Reach
    y-axis Low Engagement --> High Engagement
    quadrant-1 We should expand
    quadrant-2 Need to promote
    quadrant-3 Re-evaluate
    quadrant-4 May be improved
    Campaign A: [0.3, 0.6]
    Campaign B: [0.45, 0.23]
    Campaign C: [0.8, 0.9]
    Campaign D: [0.7, 0.3]'

# ── XY CHART TESTS ──────────────────────────────────────────────────────────

run_test "XY Chart: Sales revenue" \
  "Bar chart with line overlay. Y-axis labels and title. X-axis category labels. Bars (█) with line dots (●) connected." \
  'xychart-beta
    title "Sales Revenue"
    x-axis [jan, feb, mar, apr, may, jun]
    y-axis "Revenue" 0 --> 12000
    bar [5000, 6000, 7500, 8200, 9800, 11000]
    line [5000, 6000, 7500, 8200, 9800, 11000]'

# ── GANTT CHART TESTS ───────────────────────────────────────────────────────

run_test "Gantt: Cloud migration" \
  "5 sections with overlapping tasks. Red today marker (┆) if date is in range. Section color strips on bar area only. Bold section headers, regular task labels." \
  'gantt
    title Cloud Migration Program
    dateFormat YYYY-MM-DD
    section Discovery
        Inventory audit       :d1, 2026-03-10, 7d
        Dependency mapping    :d2, 2026-03-12, 10d
        Risk assessment       :d3, after d1, 5d
        Cost modeling         :d4, after d2, 4d
    section Infrastructure
        Network setup         :i1, after d2, 8d
        IAM policies          :i2, after d3, 6d
        Terraform modules     :i3, after i1, 12d
        Monitoring stack      :i4, after i2, 10d
    section Migration
        Database migration    :m1, after i1, 14d
        App containerization  :m2, after i3, 10d
        Data sync pipeline    :m3, after m1, 7d
        Service mesh config   :m4, after i3, 8d
    section Validation
        Load testing          :v1, after m2, 6d
        Security scan         :v2, after m1, 5d
        DR drill              :v3, after v1, 4d
        Compliance audit      :v4, after v2, 7d
    section Cutover
        DNS switchover        :c1, after v3, 2d
        Traffic migration     :c2, after c1, 3d
        Legacy decommission   :c3, after c2, 5d'

# ── ASCII MODE TEST ──────────────────────────────────────────────────────────

echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "TEST: ASCII mode: Flowchart (--ascii flag)"
echo "CHECK: All box-drawing uses +, -, | characters. Arrows use > < v ^. No Unicode characters anywhere."
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""
echo 'graph LR; A[Start] --> B{Check} --> C[Done]' | $TERMAID --ascii
echo ""

echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "TEST: ASCII mode: Pie chart (--ascii flag)"
echo "CHECK: Horizontal bar chart with different fill characters (#, *, +, etc). No circle."
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""
echo 'pie title Languages
    "Go" : 40
    "Python" : 30
    "Rust" : 20
    "Other" : 10' | $TERMAID --ascii
echo ""

echo ""
echo "════════════════════════════════════════════════════════════════════════"
echo "Visual test suite complete — $total tests [$THEME_LABEL]"
echo "════════════════════════════════════════════════════════════════════════"
