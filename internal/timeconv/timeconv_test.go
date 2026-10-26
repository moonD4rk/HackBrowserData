package timeconv

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Anchor: 2024-01-15T10:30:00Z.
const anchorUnixSeconds = int64(1705314600)

const anchorCoreDataSeconds = anchorUnixSeconds - 978307200

var (
	anchorTime           = time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)
	anchorChromiumMicros = (anchorUnixSeconds + 11644473600) * 1_000_000
)

func TestChromium_AnchorDate(t *testing.T) {
	got := Chromium(anchorChromiumMicros)
	assert.Equal(t, anchorTime, got)
	assert.Equal(t, anchorUnixSeconds, got.Unix())
}

func TestChromium_ZeroReturnsZeroTime(t *testing.T) {
	assert.True(t, Chromium(0).IsZero())
}

func TestChromium_NegativeReturnsZeroTime(t *testing.T) {
	assert.True(t, Chromium(-1).IsZero())
}

func TestChromium_AlwaysUTC(t *testing.T) {
	// assert.Same compares pointers, so it catches a dropped .UTC() even when the runner's TZ is UTC.
	assert.Same(t, time.UTC, Chromium(anchorChromiumMicros).Location())
}

func TestChromium_MicrosecondPrecisionPreserved(t *testing.T) {
	got := Chromium(anchorChromiumMicros + 123456)
	assert.Equal(t, 123456*int64(time.Microsecond), int64(got.Nanosecond()))
}

func TestChromium_UnixEpochBoundary(t *testing.T) {
	assert.Equal(t, time.Unix(0, 0).UTC(), Chromium(chromiumEpochOffsetMicros))
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

func TestUnixUnits_AlwaysUTC(t *testing.T) {
	assert.Same(t, time.UTC, UnixMicro(anchorUnixSeconds*1_000_000).Location())
	assert.Same(t, time.UTC, UnixMilli(anchorUnixSeconds*1_000).Location())
	assert.Same(t, time.UTC, Unix(anchorUnixSeconds).Location())
}

func TestUnixUnits_SameMomentAcrossUnits(t *testing.T) {
	us := UnixMicro(anchorUnixSeconds * 1_000_000)
	ms := UnixMilli(anchorUnixSeconds * 1_000)
	s := Unix(anchorUnixSeconds)
	assert.True(t, us.Equal(ms))
	assert.True(t, ms.Equal(s))
}

func TestUnixUnits_OutOfJSONRangeReturnsZero(t *testing.T) {
	for _, tc := range []struct {
		name string
		got  time.Time
	}{
		{"seconds", Unix(1 << 50)},
		{"millis", UnixMilli(1 << 60)},
		{"micros", UnixMicro(1 << 62)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			b, err := tc.got.MarshalJSON()
			require.NoError(t, err)
			assert.JSONEq(t, `"0001-01-01T00:00:00Z"`, string(b))
		})
	}
}

func TestCoreData_AnchorDate(t *testing.T) {
	assert.Equal(t, anchorTime, CoreData(float64(anchorCoreDataSeconds)))
}

func TestCoreData_KnownValue(t *testing.T) {
	assert.Equal(t, int64(1678307200), CoreData(700000000).Unix())
}

func TestCoreData_EpochZero(t *testing.T) {
	assert.True(t, CoreData(0).IsZero())
}

func TestCoreData_NegativeReturnsZeroTime(t *testing.T) {
	assert.True(t, CoreData(-1).IsZero())
}

func TestCoreData_FractionalSecondsPreserved(t *testing.T) {
	got := CoreData(float64(anchorCoreDataSeconds) + 0.5)
	assert.Equal(t, 500*int64(time.Millisecond), int64(got.Nanosecond()))
}

func TestCoreData_AlwaysUTC(t *testing.T) {
	assert.Same(t, time.UTC, CoreData(float64(anchorCoreDataSeconds)).Location())
}
