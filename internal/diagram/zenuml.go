package diagram

import (
	"regexp"
	"strings"

	"github.com/aaronsb/mmaid-go/internal/renderer"
)

// Read, against https://mermaid.js.org/syntax/zenuml.html and the ZenUML
// grammar it renders through, `sequenceParser.g4` and `sequenceLexer.g4` at
// ZenUML/core:
//   - `title <text>`;
//   - participants: a bare name, `A as Alice`, an `@Annotator Name`, and the
//     names inside a `group ... { }`;
//   - `@Starter(A)`, and the starter upstream's `ProgContext.Starter` falls
//     back to: the first statement's sender, else the first declared
//     participant, else a lane of its own;
//   - sync calls, `[Type] [x =] [From->]To.method(args)`, with a `{ }` body;
//   - creation, `[x =] new A(args)`, with a `{ }` body;
//   - async messages, `[From->]To: text`, and `From->To` with no payload;
//   - replies: `return expr`, an assignment on a call, and `@return` or
//     `@reply` on the async message that follows it;
//   - fragments: `if`/`else if`/`else`, `while`/`for`/`forEach`/`loop`,
//     `opt`, `par`, and `try`/`catch`/`finally`;
//   - `//` comments, which become a note over the participant the next
//     message reaches.
//
// Skipped:
//   - `<<stereotype>>`, a participant's pixel width and its `#rrggbb` colour:
//     the sequence model has no place for any of them.
//   - `==` dividers. The model has no divider event.
//   - a `group` frame around its participants. The lanes are drawn, the box
//     around them is not.
//   - more than one statement on a line, and a `{` block whose body shares
//     the opening line. A statement is a line here.
//   - a `//` comment that is not at the start of its line, which the lexer
//     reads as a comment and a payload after `:` does not.
//   - a comment above a fragment or a participant. Upstream renders the
//     first and ignores the second; here both are dropped, the model having
//     nowhere to hang a note that is not over a message.
//
// Divergences:
//   - An async message draws the arrowhead a sync call draws. The two are
//     told apart by the activation bar a call raises on its callee.
//   - A `try` is the sequence renderer's `critical` frame, and `catch` and
//     `finally` are its sections.

// zenName matches a participant or method name: an identifier or a quoted
// string.
const zenName = `(?:"[^"]*"|[A-Za-z_][A-Za-z_0-9]*)`

var (
	reZenTitle   = regexp.MustCompile(`(?i)^title(?:\s+(.*))?$`)
	reZenStarter = regexp.MustCompile(`(?i)^@starter\s*\(\s*(` + zenName + `)?\s*\)$`)
	reZenReplyAt = regexp.MustCompile(`(?i)^@(?:return|reply)$`)
	reZenAnnot   = regexp.MustCompile(`^@([A-Za-z_][A-Za-z_0-9]*)\s+(.+)$`)
	reZenAlias   = regexp.MustCompile(`^(` + zenName + `)\s+as\s+(.+)$`)
	reZenGroup   = regexp.MustCompile(`(?i)^group\b\s*(?:` + zenName + `)?\s*\{?$`)
	reZenIf      = regexp.MustCompile(`(?i)^if\b\s*\(?\s*(.*?)\s*\)?\s*\{?$`)
	reZenElseIf  = regexp.MustCompile(`(?i)^else\s+if\b\s*\(?\s*(.*?)\s*\)?\s*\{?$`)
	reZenElse    = regexp.MustCompile(`(?i)^else\s*\{?$`)
	reZenLoop    = regexp.MustCompile(`(?i)^(?:while|for|foreach|loop)\b\s*\(?\s*(.*?)\s*\)?\s*\{?$`)
	reZenOpt     = regexp.MustCompile(`(?i)^(opt|par)\b\s*\{?$`)
	reZenTry     = regexp.MustCompile(`(?i)^try\s*\{?$`)
	reZenCatch   = regexp.MustCompile(`(?i)^catch\b\s*\(?\s*(.*?)\s*\)?\s*\{?$`)
	reZenFinally = regexp.MustCompile(`(?i)^finally\s*\{?$`)
	reZenReturn  = regexp.MustCompile(`(?i)^return\b\s*(.*)$`)
	reZenNew     = regexp.MustCompile(`(?i)^(?:(?:` + zenName + `\s+)?(` + zenName + `)\s*=\s*)?new\s+(` + zenName + `)\s*(\(.*\))?\s*\{?$`)
	reZenAsync   = regexp.MustCompile(`^(?:(` + zenName + `)\s*->\s*)?(` + zenName + `)\s*:\s*(.*)$`)
	reZenBare    = regexp.MustCompile(`^(` + zenName + `)\s*(?:->\s*(` + zenName + `))?$`)
	reZenSync    = regexp.MustCompile(`^(?:(?:` + zenName + `\s+)?(` + zenName + `)\s*=\s*)?(?:(` + zenName + `)\s*->\s*)?(` + zenName + `)\s*\.\s*(.+)$`)
)

// zenKinds maps the annotators the sequence model draws a shape for. Every
// other annotator is a plain participant.
var zenKinds = map[string]string{
	"actor":       "actor",
	"database":    "database",
	"queue":       "queue",
	"boundary":    "boundary",
	"control":     "control",
	"entity":      "entity",
	"collections": "collections",
}

// zenParser walks the statement lines, appending to whichever event list the
// block it is inside owns.
type zenParser struct {
	d       *sequenceDiagram
	lines   []string
	i       int
	pending []string // comment lines waiting for the message they annotate
	reply   bool     // an @return annotator applies to the next async message
}

// zenLines strips comments' surroundings into statement lines: one statement
// per line, and a closing brace alone on its own.
func zenLines(source string) []string {
	var out []string
	first := true
	for _, raw := range strings.Split(source, "\n") {
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}
		if first {
			first = false
			if strings.EqualFold(line, "zenuml") {
				continue
			}
		}
		if strings.HasPrefix(line, "//") {
			out = append(out, line)
			continue
		}
		for strings.HasPrefix(line, "}") {
			out = append(out, "}")
			line = strings.TrimSpace(line[1:])
		}
		line = strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(line), ";"))
		if line != "" {
			out = append(out, line)
		}
	}
	return out
}

// zenUnquote strips the quotes a name may be written with.
func zenUnquote(s string) string {
	s = strings.TrimSpace(s)
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		return s[1 : len(s)-1]
	}
	return s
}

// zenStarter resolves who the top-level statements run from, in the order
// upstream's ProgContext.Starter resolves it.
func zenStarter(lines []string) string {
	firstParticipant := ""
	for _, line := range lines {
		if strings.HasPrefix(line, "//") || reZenTitle.MatchString(line) {
			continue
		}
		if m := reZenStarter.FindStringSubmatch(line); m != nil {
			if name := zenUnquote(m[1]); name != "" {
				return name
			}
			continue
		}
		if m := reZenAsync.FindStringSubmatch(line); m != nil {
			if m[1] != "" {
				return zenUnquote(m[1])
			}
			break
		}
		if m := reZenSync.FindStringSubmatch(line); m != nil {
			if m[2] != "" {
				return zenUnquote(m[2])
			}
			break
		}
		if name, _, _, ok := zenParticipant(line); ok {
			if firstParticipant == "" {
				firstParticipant = name
			}
			continue
		}
		break // any other statement: the head is over
	}
	if firstParticipant != "" {
		return firstParticipant
	}
	return "Starter"
}

// zenParticipant reads a participant declaration: `Bob`, `A as Alice`, or
// either behind an `@Annotator`.
func zenParticipant(line string) (id, label, kind string, ok bool) {
	kind = "participant"
	if m := reZenAnnot.FindStringSubmatch(line); m != nil {
		if k, known := zenKinds[strings.ToLower(m[1])]; known {
			kind = k
		}
		line = strings.TrimSpace(m[2])
	}
	if m := reZenAlias.FindStringSubmatch(line); m != nil {
		id, label = zenUnquote(m[1]), zenUnquote(strings.TrimSpace(m[2]))
		return id, label, kind, label != ""
	}
	if m := reZenBare.FindStringSubmatch(line); m != nil && m[2] == "" {
		id = zenUnquote(m[1])
		return id, id, kind, true
	}
	return "", "", "", false
}

// parseZenUML parses a ZenUML definition into the sequence model.
//
//	zenuml
//	    title Checkout
//	    @Actor Customer
//	    Customer->Cart: add item
//	    receipt = Cart.checkout(token) {
//	      Payments.charge(token)
//	      return receipt
//	    }
func parseZenUML(source string) *sequenceDiagram {
	p := &zenParser{d: &sequenceDiagram{}, lines: zenLines(source)}
	starter := zenStarter(p.lines)
	for p.i < len(p.lines) {
		if p.lines[p.i] == "}" { // a close with nothing open
			p.i++
			continue
		}
		p.statements(&p.d.events, starter, "")
	}
	if len(p.d.events) > 0 {
		ensureParticipant(p.d, starter)
	}
	return p.d
}

// statements consumes lines until the brace that closes the block it is
// reading, which it leaves for the opener. `self` runs the statements and
// `invoker` is who a `return` replies to. It reports whether one did.
func (p *zenParser) statements(sink *[]any, self, invoker string) bool {
	returned := false
	for p.i < len(p.lines) {
		line := p.lines[p.i]
		if line == "}" {
			return returned
		}
		p.i++
		if p.statement(sink, line, self, invoker) {
			returned = true
		}
	}
	return returned
}

// opensBlock reports whether a statement's own line opens a `{ }` body, and
// returns the statement without the brace.
func opensBlock(line string) (string, bool) {
	if trimmed, ok := strings.CutSuffix(line, "{"); ok {
		return strings.TrimSpace(trimmed), true
	}
	return line, false
}

// statement parses one line. It reports whether it replied to `invoker`.
func (p *zenParser) statement(sink *[]any, line, self, invoker string) bool {
	if strings.HasPrefix(line, "//") {
		p.pending = append(p.pending, strings.TrimSpace(strings.TrimPrefix(line, "//")))
		return false
	}
	if m := reZenTitle.FindStringSubmatch(line); m != nil {
		p.d.title = strings.TrimSpace(m[1])
		return false
	}
	if reZenStarter.MatchString(line) {
		return false
	}
	if reZenReplyAt.MatchString(line) {
		p.reply = true
		return false
	}

	// Fragments. Every one is a frame the sequence renderer already draws.
	body, _ := opensBlock(line)
	switch {
	case reZenIf.MatchString(body):
		return p.fragment(sink, "alt", reZenIf.FindStringSubmatch(body)[1], self, invoker)
	case reZenLoop.MatchString(body):
		return p.fragment(sink, "loop", reZenLoop.FindStringSubmatch(body)[1], self, invoker)
	case reZenOpt.MatchString(body):
		return p.fragment(sink, strings.ToLower(reZenOpt.FindStringSubmatch(body)[1]), "", self, invoker)
	case reZenTry.MatchString(body):
		return p.fragment(sink, "critical", "", self, invoker)
	case reZenGroup.MatchString(body):
		p.group()
		return false
	}

	if m := reZenReturn.FindStringSubmatch(line); m != nil {
		if invoker == "" {
			return false // a return at the top level has nowhere to go
		}
		p.message(sink, self, invoker, strings.TrimSpace(m[1]), "dotted")
		return true
	}
	if m := reZenNew.FindStringSubmatch(line); m != nil {
		label := "new " + zenUnquote(m[2])
		if m[3] != "" {
			label += m[3]
		}
		p.call(sink, self, zenUnquote(m[2]), label, zenUnquote(m[1]), line)
		return false
	}
	if m := reZenAsync.FindStringSubmatch(line); m != nil {
		from := zenUnquote(m[1])
		if from == "" {
			from = self
		}
		lineType := "solid"
		if p.reply {
			lineType, p.reply = "dotted", false
		}
		p.note(sink, from, zenUnquote(m[2]))
		p.message(sink, from, zenUnquote(m[2]), strings.TrimSpace(m[3]), lineType)
		return false
	}
	if m := reZenSync.FindStringSubmatch(line); m != nil {
		from := zenUnquote(m[2])
		if from == "" {
			from = self
		}
		to := zenUnquote(m[3])
		method, _ := opensBlock(strings.TrimSpace(m[4]))
		p.call(sink, from, to, method, zenUnquote(m[1]), line)
		return false
	}
	if m := reZenBare.FindStringSubmatch(line); m != nil && m[2] != "" {
		p.message(sink, zenUnquote(m[1]), zenUnquote(m[2]), "", "solid")
		return false
	}
	if id, label, kind, ok := zenParticipant(line); ok {
		p.declare(id, label, kind)
		return false
	}
	p.d.warnings = append(p.d.warnings, "Unrecognized line: "+line)
	return false
}

// declare records a participant with the label and the shape it was declared
// with.
func (p *zenParser) declare(id, label, kind string) {
	p.pending = nil // a comment on a participant is not rendered
	ensureParticipant(p.d, id)
	for _, q := range p.d.participants {
		if q.id == id {
			q.kind, q.label = kind, label
		}
	}
}

// group reads the participants a `group { }` holds. The lanes are kept and
// the frame around them is not.
func (p *zenParser) group() {
	for p.i < len(p.lines) {
		line := p.lines[p.i]
		if line == "}" {
			p.i++
			return
		}
		p.i++
		if id, label, kind, ok := zenParticipant(line); ok {
			p.declare(id, label, kind)
		}
	}
}

// note turns the comments waiting above a message into a note over the
// participant it reaches. The message's own participants are registered
// first, so a note does not reorder the lanes.
func (p *zenParser) note(sink *[]any, from, over string) {
	ensureParticipant(p.d, from)
	ensureParticipant(p.d, over)
	if len(p.pending) == 0 {
		return
	}
	*sink = append(*sink, &note{
		text:         strings.Join(p.pending, "\n"),
		position:     "over",
		participants: []string{over},
	})
	p.pending = nil
}

// message appends one arrow.
func (p *zenParser) message(sink *[]any, from, to, label, lineType string) {
	ensureParticipant(p.d, from)
	ensureParticipant(p.d, to)
	*sink = append(*sink, &message{
		source: from, target: to, label: label,
		lineType: lineType, arrowType: "arrow",
	})
}

// call draws a sync call: the arrow, the activation it raises on the callee,
// the statements of its `{ }` body, and the reply an explicit return or an
// assignment asks for.
func (p *zenParser) call(sink *[]any, from, to, label, assignee, line string) {
	p.note(sink, from, to)
	p.message(sink, from, to, label, "solid")
	*sink = append(*sink, &activateEvent{participant: to, active: true})

	returned := false
	if _, hasBody := opensBlock(line); hasBody {
		returned = p.statements(sink, to, from)
		if p.i < len(p.lines) && p.lines[p.i] == "}" {
			p.i++
		}
	}
	if !returned && assignee != "" {
		p.message(sink, to, from, assignee, "dotted")
	}
	*sink = append(*sink, &activateEvent{participant: to, active: false})
}

// fragment draws one frame and the sections that continue it.
func (p *zenParser) fragment(sink *[]any, kind, label, self, invoker string) bool {
	p.pending = nil
	blk := &block{kind: kind, label: strings.TrimSpace(label)}
	*sink = append(*sink, blk)
	returned := p.statements(&blk.events, self, invoker)

	for {
		if p.i < len(p.lines) && p.lines[p.i] == "}" {
			p.i++
		}
		if p.i >= len(p.lines) {
			break
		}
		section, ok := zenSection(p.lines[p.i])
		if !ok {
			break
		}
		p.i++
		sec := &blockSection{label: section}
		blk.sections = append(blk.sections, sec)
		if p.statements(&sec.events, self, invoker) {
			returned = true
		}
	}
	return returned
}

// zenSection reads the keyword that continues a fragment into its next
// section.
func zenSection(line string) (string, bool) {
	body, _ := opensBlock(line)
	switch {
	case reZenElseIf.MatchString(body):
		return "else " + reZenElseIf.FindStringSubmatch(body)[1], true
	case reZenElse.MatchString(body):
		return "else", true
	case reZenCatch.MatchString(body):
		return strings.TrimSpace("catch " + reZenCatch.FindStringSubmatch(body)[1]), true
	case reZenFinally.MatchString(body):
		return "finally", true
	}
	return "", false
}

// RenderZenUML parses a ZenUML definition and renders it through the sequence
// renderer, which is the diagram both syntaxes describe.
func RenderZenUML(source string, cs renderer.CharSet) *renderer.Canvas {
	return renderSequenceModel(parseZenUML(source), cs)
}
