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

	// ErrorLevel is used for undesired and unexpected events that
	// the program can recover from.
	ErrorLevel

	// FatalLevel is used for undesired and unexpected events that
	// the program cannot recover from.
	FatalLevel
)

func (l Level) String() string {
	switch l {
	case DebugLevel:
		return "DBG"
	case InfoLevel:
		return "INF"
	case WarnLevel:
		return "WRN"
	case ErrorLevel:
		return "ERR"
	case FatalLevel:
		return "FTL"
	default:
		return "???"
	}
}
