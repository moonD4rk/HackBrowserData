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
// dir's copies are on disk. Its error means the run ended early; like os.ReadDir, it still returns the Summary of what
// ran before it, with the files written, so the caller can list them.
func Execute(
	ctx context.Context, discovery Discovery, categories []category.Category, output Output, keychainConfig KeychainConfig,
) (Summary, error) {
	keychain := newLoginKeychain(keychainConfig)
	return execute(ctx, discovery, categories, output, extractorsWith(keychain), keysFromMachine(keychain))
}

// ExecuteWithKeys decrypts with a key file's keys, so it never reads this machine's secret stores.
func ExecuteWithKeys(
	ctx context.Context, discovery Discovery, categories []category.Category, output Output, keys map[browser.UserDataDirID]masterkey.Set,
) (Summary, error) {
	return execute(ctx, discovery, categories, output, extractorsWith(nil), keysFromFile(keys))
}

func execute(
	ctx context.Context, discovery Discovery, categories []category.Category, output Output, extractors extractorsFunc, keys dirKeysFunc,
) (Summary, error) {
	writer, err := export.NewWriter(output.Dir, output.Format, output.Zip)
	if err != nil {
		return Summary{}, err
	}
	summary, err := runner{writer: writer, extractors: extractors, retrieveKeys: keys, logProgress: true}.run(ctx, discovery, categories)
	closeErr := writer.Close()
	summary.Exported = writer.Output()
	return summary, errors.Join(err, closeErr)
}

type recordWriter interface {
	Write(dataCategory category.Category, records []record.Record) error
}

// runner is what every user data dir of one run shares; logProgress logs each dir as it starts.
type runner struct {
	writer       recordWriter
	extractors   extractorsFunc
	retrieveKeys dirKeysFunc
	logProgress  bool
}

func (r runner) run(ctx context.Context, discovery Discovery, categories []category.Category) (Summary, error) {
	summary := Summary{DirFailures: append([]DirFailure(nil), discovery.DirFailures...)}
	for _, dir := range discovery.Dirs {
		if err := ctx.Err(); err != nil {
			return summary, fmt.Errorf("run stopped: %w", err)
		}
		dirSummary, err := r.runDir(ctx, dir, categories)
		summary.Results = append(summary.Results, dirSummary.Results...)
		summary.DirFailures = append(summary.DirFailures, dirSummary.DirFailures...)
		if err != nil {
			return summary, err
		}
	}
	return summary, nil
}

func (r runner) runDir(ctx context.Context, dir browser.UserDataDir, categories []category.Category) (Summary, error) {
	plan := planDir(dir, categories, r.extractors)
	summary := Summary{Results: plan.skipped}
	if len(plan.extractions) == 0 {
		return summary, nil
	}
	if r.logProgress {
		log.Infof("Extracting %s...", dir.Browser.DisplayName)
	}

	snapshots, err := snapshot.Temp()
	if err != nil {
		summary.Results = append(summary.Results, failAll(plan.extractions, err)...)
		return summary, nil
	}
	defer closeSnapshot(snapshots)
	if err := snapshots.Capture(ctx, plan.files); err != nil {
		if ctx.Err() != nil {
			return summary, err
		}
		summary.Results = append(summary.Results, failAll(plan.extractions, err)...)
		return summary, nil
	}
	dirSnapshot := snapshots.Snapshot(string(dir.ID))

	keys := &dirKeys{dir: dir, dirSnapshot: dirSnapshot}
	if dir.Browser.Base == browser.Chromium {
		keys.retrieve = r.retrieveKeys
	}
	for _, extraction := range plan.extractions {
		if err := ctx.Err(); err != nil {
			return summary, fmt.Errorf("run stopped: %w", err)
		}
		profileSnapshot := browser.NewProfileSnapshot(dirSnapshot, extraction.profile, keys.get)
		result, err := r.runExtraction(ctx, extraction, profileSnapshot, dir.Browser.DisplayName)
		if err != nil {
			return summary, err
		}
		summary.Results = append(summary.Results, result)
	}
	if keys.err != nil {
		keyErr := fmt.Errorf("master key retrieval: %w", keys.err)
		summary.DirFailures = []DirFailure{{UserDataDirID: dir.ID, Err: keyErr}}
	}
	return summary, nil
}

// runExtraction exports the records emitted before an extraction failed, as they always were; its error stops the run.
func (r runner) runExtraction(
	ctx context.Context, extraction plannedExtraction, profileSnapshot browser.ProfileSnapshot, browserName string,
) (Result, error) {
	recordCollector := &collector{browserName: browserName, profileName: extraction.ProfileName}
	err := extraction.extractor.Extract(ctx, profileSnapshot, recordCollector)
	if writeErr := r.writer.Write(extraction.Category, recordCollector.records); writeErr != nil {
		return Result{}, writeErr
	}
	return recordCollector.result(extraction.Extraction, err), nil
}

func failAll(extractions []plannedExtraction, err error) []Result {
	results := make([]Result, len(extractions))
	for i, extraction := range extractions {
		results[i] = Result{Extraction: extraction.Extraction, Outcome: OutcomeError, Err: err}
	}
	return results
}

func closeSnapshot(snapshots *snapshot.Dir) {
	if err := snapshots.Close(); err != nil {
		log.Warnf("%v", err)
	}
}
