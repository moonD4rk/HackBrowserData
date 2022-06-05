package typeutil

import (
	"time"

	"golang.org/x/exp/constraints"
)

// Keys returns a slice of the keys of the map. based with go 1.18 generics
func Keys[K comparable, V any](m map[K]V) []K {
	r := make([]K, 0, len(m))
	for k := range m {
		r = append(r, k)
	}
	return r
}

func IntToBool[T constraints.Signed](a T) bool {
	switch a {
	case 0, -1:
		return false
	}
	return true
}

func Reverse[T any](s []T) []T {
	h := make([]T, len(s))
	for i := 0; i < len(s); i++ {
		h[i] = s[len(s)-i-1]
	}
	return h
}

func TimeStamp(stamp int64) time.Time {
	s := time.Unix(stamp, 0)
	if s.Local().Year() > 9999 {
		return time.Date(9999, 12, 13, 23, 59, 59, 0, time.Local)
	}
	return s
}

func TimeEpoch(epoch int64) time.Time {
	maxTime := int64(99633311740000000)
	if epoch > maxTime {
		return time.Date(2049, 1, 1, 1, 1, 1, 1, time.Local)
	}
	t := time.Date(1601, 1, 1, 0, 0, 0, 0, time.Local)
	d := time.Duration(epoch)
	for i := 0; i < 1000; i++ {
		t = t.Add(d)
	}
	return t
}
