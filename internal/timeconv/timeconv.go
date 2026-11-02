// Package timeconv converts browser timestamps to UTC; a non-positive input, which browsers write for unset, is zero.
package timeconv

import "time"

const (
	// chromiumEpochOffsetMicros is Chromium's base::Time::kTimeTToMicrosecondsOffset, from 1601-01-01 to the Unix epoch.
	chromiumEpochOffsetMicros = 11644473600000000
	coreDataEpochOffset       = 978307200
	// maxCoreDataSeconds keeps CoreData inside int64 and below year 9999 before the float conversion.
	maxCoreDataSeconds = 252423993600
)

// Chromium converts base::Time, microseconds since 1601-01-01 UTC.
func Chromium(microseconds int64) time.Time {
	if microseconds <= 0 {
		return time.Time{}
	}
	return jsonSafe(time.UnixMicro(microseconds - chromiumEpochOffsetMicros).UTC())
}

func UnixMicro(microseconds int64) time.Time {
	if microseconds <= 0 {
		return time.Time{}
	}
	return jsonSafe(time.UnixMicro(microseconds).UTC())
}

func UnixMilli(milliseconds int64) time.Time {
	if milliseconds <= 0 {
		return time.Time{}
	}
	return jsonSafe(time.UnixMilli(milliseconds).UTC())
}

func Unix(seconds int64) time.Time {
	if seconds <= 0 {
		return time.Time{}
	}
	return jsonSafe(time.Unix(seconds, 0).UTC())
}

// CoreData converts CFAbsoluteTime, seconds since 2001-01-01 UTC.
func CoreData(seconds float64) time.Time {
	if seconds <= 0 || seconds > maxCoreDataSeconds {
		return time.Time{}
	}
	whole := int64(seconds)
	fraction := seconds - float64(whole)
	nanos := int64(fraction * 1e9)
	return time.Unix(whole+coreDataEpochOffset, nanos).UTC()
}

// jsonSafe turns years outside [1, 9999], which time.Time.MarshalJSON rejects, into the zero time.
func jsonSafe(timestamp time.Time) time.Time {
	if timestamp.Year() < 1 || timestamp.Year() > 9999 {
		return time.Time{}
	}
	return timestamp
}
