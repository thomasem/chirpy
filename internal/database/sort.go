package database

import (
	"sort"

	"golang.org/x/exp/constraints"
)

type SortOrder int

const (
	Asc SortOrder = iota
	Desc
)

func sortSlice[T any, V constraints.Ordered](items []T, direction SortOrder, prop func(T) V) {
	if direction == Desc {
		sort.Slice(items, func(i, j int) bool { return prop(items[i]) > prop(items[j]) })
	} else {
		sort.Slice(items, func(i, j int) bool { return prop(items[i]) < prop(items[j]) })
	}
}
