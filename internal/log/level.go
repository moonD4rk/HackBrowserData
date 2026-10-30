package log

// Level defines all the available levels we can log at.
type Level int32

const (
	// DebugLevel is the lowest level of logging.
	// Debug logs are intended for debugging and development purposes.
	DebugLevel Level = iota + 1

	// InfoLevel is used for user-facing progress and status messages.
	InfoLevel

	// WarnLevel is used for undesired but relatively expected events,
	// which may indicate a problem.
	WarnLevel
)

func (l Level) String() string {
	switch l {
	case DebugLevel:
		return "DBG"
	case InfoLevel:
		return "INF"
	case WarnLevel:
		return "WRN"
	default:
		return "???"
	}
}
