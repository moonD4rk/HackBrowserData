package log

type logLevel int32

const (
	debugLevel logLevel = iota + 1

	infoLevel

	warnLevel
)

func (l logLevel) String() string {
	switch l {
	case debugLevel:
		return "DBG"
	case infoLevel:
		return "INF"
	case warnLevel:
		return "WRN"
	default:
		return "???"
	}
}
