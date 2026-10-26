// Package timeconv converts browser timestamps to UTC; browsers write 0 for "unset", so non-positive input is the zero time.
package timeconv

import "time"

const (
	// Chromium's base::Time::kTimeTToMicrosecondsOffset, from 1601-01-01 to the Unix epoch.
	chromiumEpochOffsetMicros = 11644473600000000
	coreDataEpochOffset       = 978307200
	// Keeps CoreData inside int64 and below year 9999 before the float conversion.
	maxCoreDataSeconds = 252423993600
)

// Chromium converts base::Time, microseconds since 1601-01-01 UTC.
func Chromium(us int64) time.Time {
	if us <= 0 {
		return time.Time{}
	}
	return jsonSafe(time.UnixMicro(us - chromiumEpochOffsetMicros).UTC())
}

func UnixMicro(us int64) time.Time {
	if us <= 0 {
		return time.Time{}
	}
	return jsonSafe(time.UnixMicro(us).UTC())
}

func UnixMilli(ms int64) time.Time {
	if ms <= 0 {
		return time.Time{}
	}
	return jsonSafe(time.UnixMilli(ms).UTC())
}

func Unix(s int64) time.Time {
	if s <= 0 {
		return time.Time{}
	}
	return jsonSafe(time.Unix(s, 0).UTC())
}

// CoreData converts CFAbsoluteTime, seconds since 2001-01-01 UTC.
func CoreData(s float64) time.Time {
	if s <= 0 || s > maxCoreDataSeconds {
		return time.Time{}
	}
	whole := int64(s)
	frac := s - float64(whole)
	nanos := int64(frac * 1e9)
	return time.Unix(whole+coreDataEpochOffset, nanos).UTC()
}

// time.Time.MarshalJSON rejects years outside [1, 9999], so such values export as the zero time.
func jsonSafe(t time.Time) time.Time {
	if t.Year() < 1 || t.Year() > 9999 {
		return time.Time{}
	}
	return t
}
