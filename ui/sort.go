package ui

import (
	"cmp"
	"slices"
)

// sortOrder represents the current sort mode for the notes list.
type sortOrder int

const (
	sortNameAsc    sortOrder = iota // A-Z by name
	sortNameDesc                    // Z-A by name
	sortModTimeNew                  // newest first
	sortModTimeOld                  // oldest first
	sortOrderCount                  // sentinel for cycling
)

func (s sortOrder) String() string {
	switch s {
	case sortNameAsc:
		return "name (A-Z)"
	case sortNameDesc:
		return "name (Z-A)"
	case sortModTimeNew:
		return "modified (newest)"
	case sortModTimeOld:
		return "modified (oldest)"
	default:
		return "unknown"
	}
}

func sortMarkdownsByOrder(mds []*markdown, order sortOrder) {
	switch order {
	case sortNameAsc:
		slices.SortStableFunc(mds, func(a, b *markdown) int {
			return cmp.Compare(a.Note, b.Note)
		})
	case sortNameDesc:
		slices.SortStableFunc(mds, func(a, b *markdown) int {
			return cmp.Compare(b.Note, a.Note)
		})
	case sortModTimeNew:
		slices.SortStableFunc(mds, func(a, b *markdown) int {
			return b.Modtime.Compare(a.Modtime)
		})
	case sortModTimeOld:
		slices.SortStableFunc(mds, func(a, b *markdown) int {
			return a.Modtime.Compare(b.Modtime)
		})
	}
}
