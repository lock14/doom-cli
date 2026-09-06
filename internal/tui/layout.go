package tui

const (
	sideBySideMinWidth = 90
	leftPanelWidth     = 44 // interior content width (42) + horizontal padding (2)
	gutterWidth        = 2  // horizontal space between panes
	boxBorderColumns   = 4  // 2 border chars on left box + 2 border chars on right box

	minSearchWidth = 20
	maxSearchWidth = 50

	minBoxWidth  = 36
	minBoxHeight = 3
)

// calculateSearchWidth returns the clamped width for the preset search input.
func calculateSearchWidth(termWidth int) int {
	w := termWidth - 45
	if w > maxSearchWidth {
		return maxSearchWidth
	}
	if w < minSearchWidth {
		return minSearchWidth
	}
	return w
}

// calculateInteriorHeight returns the shared interior box height for both the main panes and README viewer.
func calculateInteriorHeight(termHeight int) int {
	h := termHeight - 8
	if h < 5 {
		return 5
	}
	return h
}

// calculateReadmeDimensions returns the responsive outer box and viewport dimensions for the README viewer.
func calculateReadmeDimensions(termWidth, termHeight int) (boxWidth, vpWidth, vpHeight int) {
	boxWidth = termWidth - 2
	if boxWidth < minBoxWidth {
		boxWidth = minBoxWidth
	}
	vpWidth = boxWidth - 2
	if vpWidth < 36 {
		vpWidth = 36
	}
	vpHeight = calculateInteriorHeight(termHeight)
	return boxWidth, vpWidth, vpHeight
}

type layoutGeometry struct {
	sideBySide     bool
	maxVisible     int
	interiorHeight int
	listHeight     int
	detailsHeight  int
	leftWidth      int
	rightWidth     int
	boxWidth       int
	gutter         int
}

func (m model) computeLayout() layoutGeometry {
	interiorHeight := calculateInteriorHeight(m.height)
	if m.width >= sideBySideMinWidth {
		rightWidth := m.width - leftPanelWidth - gutterWidth - boxBorderColumns
		if rightWidth < 38 {
			rightWidth = 38
		}
		return layoutGeometry{
			sideBySide:     true,
			maxVisible:     interiorHeight,
			interiorHeight: interiorHeight,
			leftWidth:      leftPanelWidth,
			rightWidth:     rightWidth,
			gutter:         gutterWidth,
		}
	}

	boxWidth := m.width - 2
	if boxWidth < minBoxWidth {
		boxWidth = minBoxWidth
	}
	availHeight := m.height - 5
	if availHeight < 6 {
		availHeight = 6
	}
	listHeight := availHeight / 2
	if listHeight < minBoxHeight {
		listHeight = minBoxHeight
	}
	detailsHeight := availHeight - listHeight
	if detailsHeight < minBoxHeight {
		detailsHeight = minBoxHeight
	}
	maxVisible := listHeight - 2
	if maxVisible < 1 {
		maxVisible = 1
	}

	return layoutGeometry{
		sideBySide:    false,
		maxVisible:    maxVisible,
		listHeight:    listHeight,
		detailsHeight: detailsHeight,
		boxWidth:      boxWidth,
	}
}
