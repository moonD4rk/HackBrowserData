package firefox

import "time"

// timestamp converts a Unix epoch timestamp (seconds) to a time.Time.
func timestamp(stamp int64) time.Time {
	s := time.Unix(stamp, 0)
	if s.Local().Year() > 9999 {
		return time.Date(9999, 12, 13, 23, 59, 59, 0, time.Local)
	}
	return s
}
