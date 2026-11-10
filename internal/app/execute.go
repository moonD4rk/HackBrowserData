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
	fromMachine := runner{extractors: extractorsFor, retrieveKeys: keysFromMachine(keychain), loginKeychain: keychain}
	return execute(ctx, discovery, categories, output, fromMachine)
}

// ExecuteWithKeys decrypts with a key file's keys, so it never reads this machine's secret stores.
func ExecuteWithKeys(
	ctx context.Context, discovery Discovery, categories []category.Category, output Output, keys map[browser.UserDataDirID]masterkey.Set,
) (Summary, error) {
	return execute(ctx, discovery, categories, output, runner{extractors: extractorsFor, retrieveKeys: keysFromFile(keys)})
}

// execute gives the runner the run's writer; the caller sets where its extractors, keys, and keychain come from.
func execute(ctx context.Context, discovery Discovery, categories []category.Category, output Output, dirRunner runner) (Summary, error) {
	writer, err := export.NewWriter(output.Dir, output.Format, output.Zip)
	if err != nil {
		return Summary{}, err
	}
	dirRunner.writer, dirRunner.logProgress = writer, true
	summary, err := dirRunner.run(ctx, discovery, categories)
	closeErr := writer.Close()
	summary.Exported = writer.Listing()
	return summary, errors.Join(err, closeErr)
}

type recordWriter interface {
	Write(dataCategory category.Category, records []record.Record) error
}

// runner is what every user data dir of one run shares; logProgress logs each dir as it starts.
type runner struct {
	writer        recordWriter
	extractors    extractorsFunc
	retrieveKeys  dirKeysFunc
	loginKeychain browser.LoginKeychain
	logProgress   bool
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

	root, err := snapshot.Temp()
	if err != nil {
		summary.Results = append(summary.Results, failAll(plan.extractions, err)...)
		return summary, nil
	}
	defer closeRoot(root)
	if err := root.Capture(ctx, plan.files); err != nil {
		if ctx.Err() != nil {
			return summary, err
		}
		summary.Results = append(summary.Results, failAll(plan.extractions, err)...)
		return summary, nil
	}
	dirSnapshot := root.Snapshot(string(dir.ID))

	keys := &dirKeys{dir: dir, dirSnapshot: dirSnapshot}
	if dir.Browser.Base == browser.Chromium {
		keys.retrieve = r.retrieveKeys
	}
	for _, extraction := range plan.extractions {
		if err := ctx.Err(); err != nil {
			return summary, fmt.Errorf("run stopped: %w", err)
		}
		input := browser.NewInput(dirSnapshot, extraction.profile, keys.get, r.loginKeychain)
		result, err := r.runExtraction(ctx, extraction, input, dir.Browser.DisplayName)
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
	ctx context.Context, extraction plannedExtraction, input browser.Input, browserName string,
) (Result, error) {
	recordCollector := &collector{browserName: browserName, profileName: extraction.ProfileName}
	err := extraction.extractor.Extract(ctx, input, recordCollector)
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

func closeRoot(root *snapshot.Root) {
	if err := root.Close(); err != nil {
		log.Warnf("%v", err)
	}
}
