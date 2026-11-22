package app

import (
	"errors"
	"fmt"
	"io/fs"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/moond4rk/hackbrowserdata/internal/category"
	"github.com/moond4rk/hackbrowserdata/internal/record"
)

func TestCollectorResult(t *testing.T) {
	notDecrypted := errors.New("value not decrypted")
	corrupt := errors.New("corrupt")
	gone := fmt.Errorf("History: %w", fs.ErrNotExist)
	extraction := Extraction{UserDataDirID: "chrome", ProfileName: "Default", Category: category.Cookie}
	tests := []struct {
		name     string
		records  int
		failures int
		err      error
		want     Outcome
		wantErr  error
	}{
		{name: "an empty file is ok, not absent", want: OutcomeOK},
		{name: "records", records: 2, want: OutcomeOK},
		{name: "problems beside records", records: 2, failures: 1, want: OutcomePartial},
		{name: "problems and no record", failures: 2, want: OutcomeError, wantErr: notDecrypted},
		{name: "a failed extraction", err: corrupt, want: OutcomeError, wantErr: corrupt},
		{name: "a failed extraction keeps its records", records: 1, err: corrupt, want: OutcomeError, wantErr: corrupt},
		{name: "a file that vanished after planning", err: gone, want: OutcomeAbsent},
		{name: "a file gone after records were read", records: 1, err: gone, want: OutcomeError, wantErr: gone},
	}
	for _, testCase := range tests {
		recordCollector := &collector{}
		for i := 0; i < testCase.records; i++ {
			recordCollector.Add(&record.Cookie{})
		}
		for i := 0; i < testCase.failures; i++ {
			recordCollector.Fail(notDecrypted)
		}

		got := recordCollector.result(extraction, testCase.err)

		want := Result{
			Extraction: extraction, Outcome: testCase.want,
			Records: testCase.records, Failures: testCase.failures, Err: testCase.wantErr,
		}
		assert.Equal(t, want, got, testCase.name)
	}
}
