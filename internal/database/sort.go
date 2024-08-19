package database

import (
	"sort"

	"golang.org/x/exp/constraints"
)

type SortDirection int

const (
	Asc SortDirection = iota
	Desc
)

func sortSlice[T any, V constraints.Ordered](items []T, direction SortDirection, prop func(T) V) {
	lessFn := func(i, j int) bool {
		if direction == Desc {
			return prop(items[i]) > prop(items[j])
		}
		return prop(items[i]) < prop(items[j])
	}
	sort.Slice(items, lessFn)
}
