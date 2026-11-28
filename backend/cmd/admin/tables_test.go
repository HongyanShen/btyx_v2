package main

import (
	"slices"
	"testing"
)

func TestPagerNumbers(t *testing.T) {
	cases := []struct {
		name    string
		current int
		last    int
		want    []int
	}{
		{
			name:    "single page",
			current: 1,
			last:    1,
			want:    []int{1},
		},
		{
			name:    "first page of a large list",
			current: 1,
			last:    40,
			want:    []int{1, 2, 3, 4, 5, 0, 40},
		},
		{
			name:    "middle page of a large list",
			current: 20,
			last:    40,
			want:    []int{1, 0, 18, 19, 20, 21, 22, 0, 40},
		},
		{
			name:    "last page of a large list",
			current: 40,
			last:    40,
			want:    []int{1, 0, 36, 37, 38, 39, 40},
		},
	}
	for _, item := range cases {
		t.Run(item.name, func(t *testing.T) {
			if got := pagerNumbers(item.current, item.last); !slices.Equal(got, item.want) {
				t.Fatalf("pagerNumbers(%d, %d) = %v, want %v", item.current, item.last, got, item.want)
			}
		})
	}
}
