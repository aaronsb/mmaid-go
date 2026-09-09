# Gallery

Every fixture in `testdata/fixtures`, rendered from its reference frame in
`testdata/golden`. `make gallery` rebuilds this page; `make golden-record`
rebuilds it after re-recording the references. A fixture's first line may
carry a `%% mmaid:` directive naming the flags it renders with; the default
is `-t default -w 120`.

## block

Rendered with `-t default -w 120`.

```mermaid
block-beta
    columns 3
    A["Frontend"] B["API"] C["Database"]
    D["Cache"]:2 E["Queue"]
```

![block](block.png)

## c4

Rendered with `-t default -w 120`.

```mermaid
C4Context
    title Internet Banking
    Enterprise_Boundary(b0, "Bank") {
        Person(customer, "Banking Customer", "A personal account holder.")
        Person(staff, "Support Staff", "Answers customer queries.")
        System(banking, "Internet Banking", "Accounts and payments.")
    }
    System_Ext(email, "E-mail System", "Microsoft Exchange.")

    Rel(customer, banking, "Uses")
    Rel(staff, banking, "Administers")
    Rel(banking, email, "Sends mail", "SMTP")
```

![c4](c4.png)

## class

Rendered with `-t mono`.

```mermaid
classDiagram
    class Animal {
        +String name
        +int age
        +makeSound()
    }
    class Dog {
        +fetch()
    }
    class Cat {
        +purr()
    }
    Animal <|-- Dog
    Animal <|-- Cat
```

![class](class.png)

## cynefin

Rendered with `-t default -w 120`.

```mermaid
cynefin-beta
    title Incident Response
    complex
      "Investigate root cause"
      "Run a chaos experiment"
    complicated
      "Analyse performance data"
      "Expert review needed"
    clear
      "Restart the service"
      "Apply the known fix"
    chaotic
      "Page on-call immediately"
    confusion
      "Unknown failure mode"
    complex --> complicated : "Pattern identified"
    clear --> chaotic : "Complacency"
```

![cynefin](cynefin.png)

## edge-styles

Rendered with `-t default -w 120`.

```mermaid
graph LR
    A -.-> B
    B ==> C
    C --- D
    D <--> E
```

![edge-styles](edge-styles.png)

## edge-styles-ascii

Rendered with `-a`.

```mermaid
graph LR
    A -.-> B
    B ==> C
    C --- D
    D <--> E
```

![edge-styles-ascii](edge-styles-ascii.png)

## er

Rendered with `-a`.

```mermaid
erDiagram
    CUSTOMER ||--o{ ORDER : places
    ORDER ||--|{ LINE_ITEM : contains
    PRODUCT ||--o{ LINE_ITEM : "ordered in"
```

![er](er.png)

## eventmodeling

Rendered with `-t default -w 120`.

```mermaid
eventmodeling
    tf 01 ui CartUI
    tf 02 cmd AddItem ->> 01
    tf 03 evt ItemAdded ->> 02
    tf 04 rmo CartView ->> 03
    tf 05 ui CartScreen ->> 04
```

![eventmodeling](eventmodeling.png)

## flowchart

Rendered with `-t blueprint`.

```mermaid
graph LR
    A[Request] --> B{Auth?}
    B -->|Yes| C[Process]
    B -->|No| D[Reject]
    C --> E[Response]
```

![flowchart](flowchart.png)

## flowchart-ascii

Rendered with `-a -t blueprint`.

```mermaid
graph LR
    A[Request] --> B{Auth?}
    B -->|Yes| C[Process]
    B -->|No| D[Reject]
    C --> E[Response]
```

![flowchart-ascii](flowchart-ascii.png)

## flowchart-cross

Rendered with `-t default -w 120`.

```mermaid
graph LR
    A --> B
    A --> C
    A --> D
    B --> E
    C --> E
    D --> E
    E --> A
    B --> D
```

![flowchart-cross](flowchart-cross.png)

## gantt

Rendered with `-t default -w 120`.

```mermaid
gantt
    title Sprint Plan
    dateFormat YYYY-MM-DD
    section Backend
        API endpoints    :a1, 2026-03-17, 10d
        Database work    :a2, 2026-03-20, 7d
    section Frontend
        UI components    :b1, 2026-03-19, 12d
    section QA
        Testing          :c1, after a2, 8d
```

![gantt](gantt.png)

## gantt-ascii

Rendered with `-a`.

```mermaid
gantt
    title Sprint Plan
    dateFormat YYYY-MM-DD
    section Backend
        API endpoints    :a1, 2026-03-17, 10d
        Database work    :a2, 2026-03-20, 7d
    section Frontend
        UI components    :b1, 2026-03-19, 12d
    section QA
        Testing          :c1, after a2, 8d
```

![gantt-ascii](gantt-ascii.png)

## gitgraph

Rendered with `-t default -w 120`.

```mermaid
gitGraph
    commit
    commit
    branch develop
    checkout develop
    commit
    commit
    checkout main
    merge develop
    commit
```

![gitgraph](gitgraph.png)

## glyph-samples

Rendered with `--glyphs-sample`.

```mermaid

```

![glyph-samples](glyph-samples.png)

## glyph-samples-legacy

Rendered with `--glyphs-sample --glyphs legacy`.

```mermaid

```

![glyph-samples-legacy](glyph-samples-legacy.png)

## ishikawa

Rendered with `-t default -w 120`.

```mermaid
ishikawa-beta
    Late Delivery
        Process
            Slow handoffs
            Manual steps
        People
            Understaffed
        Tooling
            Flaky CI
            No cache
        Environment
            Remote timezones
```

![ishikawa](ishikawa.png)

## journey

Rendered with `-t default -w 120`.

```mermaid
journey
    title Ship a Feature
    section Build
        Write code   : 4: Dev
        Run tests    : 3: Dev, CI
    section Release
        Code review  : 2: Dev, Lead
        Deploy       : 5: Dev
```

![journey](journey.png)

## kanban

Rendered with `-t default -w 120`.

```mermaid
kanban
  col1[Backlog]
    t1[Design API]
    t2[Write tests]
  col2[In Progress]
    t3[Build parser]
  col3[Done]
    t4[Setup CI]
```

![kanban](kanban.png)

## mindmap

Rendered with `-t default -w 120`.

```mermaid
mindmap
  root((System))
    Frontend
      React
      Tailwind
    Backend
      Go
      PostgreSQL
```

![mindmap](mindmap.png)

## packet

Rendered with `-t default -w 120`.

```mermaid
packet-beta
    0-15: "Source Port"
    16-31: "Destination Port"
    32-63: "Sequence Number"
    64-95: "Acknowledgment Number"
    96-99: "Data Offset"
    100-111: "Flags"
    112-127: "Window"
```

![packet](packet.png)

## pie

Rendered with `-t default -w 120`.

```mermaid
pie title Resource Allocation
    "Compute" : 45
    "Storage" : 25
    "Network" : 15
    "Security" : 10
    "Other" : 5
```

![pie](pie.png)

## quadrant

Rendered with `-t default -w 120`.

```mermaid
quadrantChart
    title Priority Matrix
    x-axis Low Effort --> High Effort
    y-axis Low Impact --> High Impact
    quadrant-1 Do First
    quadrant-2 Schedule
    quadrant-3 Delegate
    quadrant-4 Eliminate
    Feature A: [0.2, 0.8]
    Feature B: [0.7, 0.9]
    Feature C: [0.8, 0.3]
    Feature D: [0.3, 0.4]
```

![quadrant](quadrant.png)

## radar

Rendered with `-t default -w 120`.

```mermaid
radar-beta
    title Service Comparison
    axis speed["Speed"], cost["Cost"], reliability["Reliability"]
    axis support["Support"], features["Features"]
    curve a["Vendor A"]{85, 60, 90, 70, 75}
    curve b["Vendor B"]{60, 90, 70, 85, 65}
    max 100
```

![radar](radar.png)

## radar-ascii

Rendered with `-a`.

```mermaid
radar-beta
    title Service Comparison
    axis speed["Speed"], cost["Cost"], reliability["Reliability"]
    axis support["Support"], features["Features"]
    curve a["Vendor A"]{85, 60, 90, 70, 75}
    curve b["Vendor B"]{60, 90, 70, 85, 65}
    max 100
```

![radar-ascii](radar-ascii.png)

## requirement

Rendered with `-t default -w 120`.

```mermaid
requirementDiagram
    requirement checkout_req {
    id: 1
    text: Orders must be payable online.
    risk: high
    verifymethod: test
    }

    functionalRequirement payment_req {
    id: 1.1
    text: Card payments must be authorised.
    risk: high
    verifymethod: test
    }

    element checkout_service {
    type: service
    docref: docs/checkout.md
    }

    checkout_req - contains -> payment_req
    checkout_service - satisfies -> payment_req
```

![requirement](requirement.png)

## sequence

Rendered with `-t blueprint`.

```mermaid
sequenceDiagram
    participant Client
    participant API
    participant DB
    Client->>API: GET /users
    API->>DB: SELECT *
    DB-->>API: rows
    API-->>Client: 200 OK
```

![sequence](sequence.png)

## state

Rendered with `-t blueprint`.

```mermaid
stateDiagram-v2
    [*] --> Idle
    Idle --> Processing : submit
    Processing --> Review : complete
    Review --> Idle : reject
    Review --> Done : approve
    Done --> [*]
```

![state](state.png)

## state-label

Rendered with `-t default -w 120`.

```mermaid
stateDiagram-v2
    [*] --> Idle
    Idle --> Run : start
    Run --> Idle : stop
```

![state-label](state-label.png)

## subgraph-cross

Rendered with `-t default -w 120`.

```mermaid
graph LR
    subgraph S1
        A --> B
    end
    subgraph S2
        C --> D
    end
    A --> C
    B --> D
```

![subgraph-cross](subgraph-cross.png)

## timeline

Rendered with `-t default -w 120`.

```mermaid
timeline
    title Project Milestones
    2024 Q1 : Requirements
    2024 Q2 : Design : Prototype
    2024 Q3 : Development
    2024 Q4 : Launch
```

![timeline](timeline.png)

## treemap

Rendered with `-t default -w 120`.

```mermaid
treemap-beta
    "Services"
        "API": 40
        "Web": 30
        "Worker": 20
    "Infra"
        "DB": 25
        "Cache": 15
```

![treemap](treemap.png)

## treeview

Rendered with `-t default -w 120`.

```mermaid
treeView-beta
    mmaid-go/
        cmd/
            mmaid/
                main.go
        internal/
            diagram/  ## one file per type
            renderer/
        README.md
```

![treeview](treeview.png)

## usecase

Rendered with `-t default -w 120`.

```mermaid
usecase-beta
    actor Customer("Customer")
    actor Agent("Support Agent")
    systemBoundary Storefront
        Browse("Browse catalogue")
        Checkout("Place order")
        Pay("Take payment")
    end
    Customer --> Browse
    Customer --> Checkout
    Agent --> Checkout
    Checkout ..> : include Pay
```

![usecase](usecase.png)

## venn

Rendered with `-t default -w 120`.

```mermaid
venn-beta
    title What makes a good feature
    set Desirable
    set Feasible
    set Viable
    union Desirable,Feasible["Buildable"]
    union Feasible,Viable["Sustainable"]
    union Desirable,Viable["Marketable"]
    union Desirable,Feasible,Viable["Ship it"]
```

![venn](venn.png)

## venn-ascii

Rendered with `-a`.

```mermaid
venn-beta
    title What makes a good feature
    set Desirable
    set Feasible
    set Viable
    union Desirable,Feasible["Buildable"]
    union Feasible,Viable["Sustainable"]
    union Desirable,Viable["Marketable"]
    union Desirable,Feasible,Viable["Ship it"]
```

![venn-ascii](venn-ascii.png)

## wardley

Rendered with `-t default -w 120`.

```mermaid
wardley-beta
    title Tea Shop Value Chain
    anchor Business [0.95, 0.63]
    component Cup of Tea [0.79, 0.61]
    component Tea [0.63, 0.81]
    component Hot Water [0.52, 0.80]
    component Kettle [0.43, 0.35] (buy)
    component Power [0.10, 0.70] (market)
    Business -> Cup of Tea
    Cup of Tea -> Tea
    Cup of Tea -> Hot Water
    Hot Water -> Kettle
    Kettle -> Power
    evolve Kettle 0.62
    note "Standard power lets kettles evolve" [0.30, 0.20]
```

![wardley](wardley.png)

## wide-label

Rendered with `-t default -w 120`.

```mermaid
graph LR
    A[日本語テキスト] --> B[emoji 🚀 ok]
```

![wide-label](wide-label.png)

## xychart

Rendered with `-t default -w 120`.

```mermaid
xychart-beta
    title "Monthly Revenue"
    x-axis [Jan, Feb, Mar, Apr, May]
    y-axis "Revenue ($K)" 0 --> 100
    bar [45, 52, 68, 73, 91]
    line [45, 52, 68, 73, 91]
```

![xychart](xychart.png)
