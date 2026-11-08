package app

import (
	"context"
	"errors"
	"fmt"

	"github.com/moond4rk/hackbrowserdata/internal/browser"
	"github.com/moond4rk/hackbrowserdata/internal/category"
	"github.com/moond4rk/hackbrowserdata/internal/export"
	"github.com/moond4rk/hackbrowserdata/internal/log"
	"github.com/moond4rk/hackbrowserdata/internal/masterkey"
	"github.com/moond4rk/hackbrowserdata/internal/record"
	"github.com/moond4rk/hackbrowserdata/internal/snapshot"
)

// Execute retrieves each dir's master keys from this machine. It handles one user data dir at a time, so at most one
// dir's copies are on disk. Its error means the run could not finish; every other failure is in the Summary.
func Execute(ctx context.Context, plan Plan, output Output, keychainConfig KeychainConfig) (Summary, error) {
	keychain := newLoginKeychain(keychainConfig)
	return execute(ctx, plan, output, extractorsWith(keychain), keysFromMachine(keychain))
}

// ExecuteWithKeys decrypts with a key file's keys, so it never reads this machine's secret stores.
func ExecuteWithKeys(ctx context.Context, plan Plan, output Output, keys map[browser.UserDataDirID]masterkey.Set) (Summary, error) {
	return execute(ctx, plan, output, extractorsWith(nil), keysFromFile(keys))
}

func execute(ctx context.Context, plan Plan, output Output, extractors extractorsFunc, keys dirKeysFunc) (Summary, error) {
	writer, err := export.NewWriter(output.Dir, output.Format, output.Zip)
	if err != nil {
		return Summary{}, err
	}
	summary, err := runner{writer: writer, extractors: extractors, keys: keys, logProgress: true}.run(ctx, plan)
	closeErr := writer.Close()
	summary.Exported = writer.Output()
	return summary, errors.Join(err, closeErr)
}

type recordWriter interface {
	Write(dataCategory category.Category, records []record.Record) error
}

// runner is what every user data dir of one run shares; logProgress logs each dir as it starts.
type runner struct {
	writer      recordWriter
	extractors  extractorsFunc
	keys        dirKeysFunc
	logProgress bool
}

func (r runner) run(ctx context.Context, plan Plan) (Summary, error) {
	summary := Summary{
		Results:     append([]Result(nil), plan.Skipped...),
		DirFailures: append([]DirFailure(nil), plan.DirFailures...),
	}
	for _, dirPlan := range plan.Dirs {
		if err := ctx.Err(); err != nil {
			return summary, fmt.Errorf("run stopped: %w", err)
		}
		dirSummary, err := r.runDir(ctx, dirPlan)
		summary.Results = append(summary.Results, dirSummary.Results...)
		summary.DirFailures = append(summary.DirFailures, dirSummary.DirFailures...)
		if err != nil {
			return summary, err
		}
	}
	return summary, nil
}

func (r runner) runDir(ctx context.Context, dirPlan DirPlan) (Summary, error) {
	dir := dirPlan.Dir
	if r.logProgress {
		log.Infof("Extracting %s...", dir.Browser.DisplayName)
	}

	snapshots, err := snapshot.Temp()
	if err != nil {
		return Summary{Results: failAll(dirPlan, err)}, nil
	}
	defer closeSnapshot(snapshots)
	if err := snapshots.Capture(ctx, dirPlan.Files); err != nil {
		if ctx.Err() != nil {
			return Summary{}, err
		}
		return Summary{Results: failAll(dirPlan, err)}, nil
	}
	dirSnapshot := snapshots.Snapshot(string(dir.ID))

	var (
		dirKeyRetrieval *dirKeys
		masterKeys      func(context.Context) masterkey.Set
	)
	if dir.Browser.Base == browser.Chromium {
		dirKeyRetrieval = &dirKeys{retrieve: r.keys, dir: dir, dirSnapshot: dirSnapshot}
		masterKeys = dirKeyRetrieval.get
	}

	var summary Summary
	for _, profilePlan := range dirPlan.Profiles {
		profile := profilePlan.Profile
		extractors := r.extractors(dir, profile)
		profileSnapshot := browser.NewProfileSnapshot(dirSnapshot, profile, masterKeys)
		for _, dataCategory := range profilePlan.Categories {
			if err := ctx.Err(); err != nil {
				return summary, fmt.Errorf("run stopped: %w", err)
			}
			extraction := Extraction{UserDataDirID: dir.ID, ProfileName: profile.Name, Category: dataCategory}
			result, err := r.runExtraction(ctx, extraction, extractors[dataCategory], profileSnapshot, dir.Browser.DisplayName)
			if err != nil {
				return summary, err
			}
			summary.Results = append(summary.Results, result)
		}
	}
	if dirKeyRetrieval != nil && dirKeyRetrieval.err != nil {
		keyErr := fmt.Errorf("master key retrieval: %w", dirKeyRetrieval.err)
		summary.DirFailures = []DirFailure{{UserDataDirID: dir.ID, Err: keyErr}}
	}
	return summary, nil
}

// runExtraction exports the records emitted before an extraction failed, as they always were; its error stops the run.
func (r runner) runExtraction(
	ctx context.Context, extraction Extraction, extractor browser.Extractor, profileSnapshot browser.ProfileSnapshot, browserName string,
) (Result, error) {
	recordCollector := &collector{browserName: browserName, profileName: extraction.ProfileName}
	err := extractor.Extract(ctx, profileSnapshot, recordCollector)
	if writeErr := r.writer.Write(extraction.Category, recordCollector.records); writeErr != nil {
		return Result{}, writeErr
	}
	return recordCollector.result(extraction, err), nil
}

func failAll(dirPlan DirPlan, err error) []Result {
	var results []Result
	for _, profilePlan := range dirPlan.Profiles {
		for _, dataCategory := range profilePlan.Categories {
			extraction := Extraction{UserDataDirID: dirPlan.Dir.ID, ProfileName: profilePlan.Profile.Name, Category: dataCategory}
			results = append(results, Result{Extraction: extraction, Outcome: OutcomeError, Err: err})
		}
	}
	return results
}

func closeSnapshot(snapshots *snapshot.Dir) {
	if err := snapshots.Close(); err != nil {
		log.Warnf("%v", err)
	}
}
