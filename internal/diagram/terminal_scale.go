package diagram

// maxScaledWidth is the upper bound for scalable chart elements (bars, plots, gaps).
// Prevents diagrams from looking unnaturally stretched on very wide terminals.
const maxScaledWidth = 140

// diagramWidth is the effective width for diagram rendering.
// Defaults to 80, overridden via CLI --width flag.
var diagramWidth int

// SetWidthOverride sets the diagram rendering width.
func SetWidthOverride(w int) {
	diagramWidth = w
}

// UsableWidth returns the diagram rendering width.
// Returns the override if set, otherwise the detected terminal width.
// Exported so the root package can pass it to the graph layout engine.
func UsableWidth() int {
	return usableWidth()
}

func usableWidth() int {
	if diagramWidth > 0 {
		return diagramWidth
	}
	return getTerminalWidth()
}

// scaleWidth adjusts a fixed-width element to use available diagram space.
// fixedOverhead is the non-scalable portion (labels, borders, margins).
// Returns the width clamped between minW and maxW.
func scaleWidth(fixedOverhead, minW, maxW int) int {
	available := usableWidth() - fixedOverhead
	if available < minW {
		return minW
	}
	if available > maxW {
		return maxW
	}
	return available
}

// scaleGap distributes available diagram width across nGaps gaps.
// naturalWidth is the diagram's natural width at current gap spacing.
// Returns adjusted gap clamped between minGap and maxGap.
func scaleGap(baseGap, nGaps, naturalWidth, minGap, maxGap int) int {
	if nGaps <= 0 {
		return baseGap
	}
	slack := usableWidth() - naturalWidth
	perGap := slack / nGaps
	newGap := baseGap + perGap
	if newGap < minGap {
		return minGap
	}
	if newGap > maxGap {
		return maxGap
	}
	return newGap
}
