package log

type Level int32

const (
	DebugLevel Level = iota + 1

	InfoLevel

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
