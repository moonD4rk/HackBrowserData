package timeconv

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// anchorUnixSeconds is 2024-01-15T10:30:00Z.
const anchorUnixSeconds = int64(1705314600)

const anchorCoreDataSeconds = anchorUnixSeconds - 978307200

var (
	anchorTime           = time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)
	anchorChromiumMicros = (anchorUnixSeconds + 11644473600) * 1_000_000
)

func TestChromium_AnchorDate(t *testing.T) {
	assert.Equal(t, anchorTime, Chromium(anchorChromiumMicros))
}

func TestChromium_ZeroReturnsZeroTime(t *testing.T) {
	assert.True(t, Chromium(0).IsZero())
}

func TestChromium_NegativeReturnsZeroTime(t *testing.T) {
	assert.True(t, Chromium(-1).IsZero())
}

func TestChromium_MicrosecondPrecisionPreserved(t *testing.T) {
	got := Chromium(anchorChromiumMicros + 123456)
	assert.Equal(t, 123456*int64(time.Microsecond), int64(got.Nanosecond()))
}

func TestChromium_OutOfJSONRangeReturnsZero(t *testing.T) {
	jsonBytes, err := Chromium(1 << 62).MarshalJSON()
	require.NoError(t, err)
	assert.JSONEq(t, `"0001-01-01T00:00:00Z"`, string(jsonBytes))
}

func TestUnixMicro_AnchorDate(t *testing.T) {
	assert.Equal(t, anchorTime, UnixMicro(anchorUnixSeconds*1_000_000))
}

func TestUnixMicro_PrecisionPreserved(t *testing.T) {
	got := UnixMicro(anchorUnixSeconds*1_000_000 + 123456)
	assert.Equal(t, 123456*int64(time.Microsecond), int64(got.Nanosecond()))
}

func TestUnixMilli_AnchorDate(t *testing.T) {
	assert.Equal(t, anchorTime, UnixMilli(anchorUnixSeconds*1_000))
}

func TestUnixMilli_PrecisionPreserved(t *testing.T) {
	got := UnixMilli(anchorUnixSeconds*1_000 + 789)
	assert.Equal(t, 789*int64(time.Millisecond), int64(got.Nanosecond()))
}

func TestUnix_AnchorDate(t *testing.T) {
	assert.Equal(t, anchorTime, Unix(anchorUnixSeconds))
}

func TestUnix_LastJSONSecond(t *testing.T) {
	assert.Equal(t, time.Date(9999, 12, 31, 23, 59, 59, 0, time.UTC), Unix(253402300799))
	assert.True(t, Unix(253402300800).IsZero(), "year 10000 is past what time.Time.MarshalJSON accepts")
}

func TestUnixUnits_ZeroReturnsZeroTime(t *testing.T) {
	assert.True(t, UnixMicro(0).IsZero(), "micros")
	assert.True(t, UnixMilli(0).IsZero(), "millis")
	assert.True(t, Unix(0).IsZero(), "seconds")
}

func TestUnixUnits_NegativeReturnsZeroTime(t *testing.T) {
	assert.True(t, UnixMicro(-1).IsZero(), "micros")
	assert.True(t, UnixMilli(-1).IsZero(), "millis")
	assert.True(t, Unix(-1).IsZero(), "seconds")
}

func TestUnixUnits_OutOfJSONRangeReturnsZero(t *testing.T) {
	for _, testCase := range []struct {
		name string
		got  time.Time
	}{
		{"seconds", Unix(1 << 50)},
		{"millis", UnixMilli(1 << 60)},
		{"micros", UnixMicro(1 << 62)},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			encoded, err := testCase.got.MarshalJSON()
			require.NoError(t, err)
			assert.JSONEq(t, `"0001-01-01T00:00:00Z"`, string(encoded))
		})
	}
}

func TestCoreData_AnchorDate(t *testing.T) {
	assert.Equal(t, anchorTime, CoreData(float64(anchorCoreDataSeconds)))
}

func TestCoreData_EpochZero(t *testing.T) {
	assert.True(t, CoreData(0).IsZero())
}

func TestCoreData_NegativeReturnsZeroTime(t *testing.T) {
	assert.True(t, CoreData(-1).IsZero())
}

func TestCoreData_OutOfJSONRangeReturnsZero(t *testing.T) {
	assert.True(t, CoreData(maxCoreDataSeconds).IsZero(), "10000-01-01 is past what time.Time.MarshalJSON accepts")
	encoded, err := CoreData(maxCoreDataSeconds - 1).MarshalJSON()
	require.NoError(t, err)
	assert.JSONEq(t, `"9999-12-31T23:59:59Z"`, string(encoded))
}

func TestCoreData_FractionalSecondsPreserved(t *testing.T) {
	got := CoreData(float64(anchorCoreDataSeconds) + 0.5)
	assert.Equal(t, 500*int64(time.Millisecond), int64(got.Nanosecond()))
}
