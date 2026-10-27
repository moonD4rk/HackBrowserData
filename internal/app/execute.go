package app

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"sync"

	"github.com/moond4rk/hackbrowserdata/internal/browser"
	"github.com/moond4rk/hackbrowserdata/internal/browser/chromium"
	"github.com/moond4rk/hackbrowserdata/internal/export"
	"github.com/moond4rk/hackbrowserdata/internal/log"
	"github.com/moond4rk/hackbrowserdata/internal/masterkey"
	"github.com/moond4rk/hackbrowserdata/internal/record"
	"github.com/moond4rk/hackbrowserdata/internal/snapshot"
)

// keySource retrieves one user data dir's Chromium keys, reading what it needs from the dir's snapshot.
type keySource func(ctx context.Context, dir browser.UserDataDir, snap snapshot.Snapshot) (masterkey.Set, error)

// Execute runs Capture, Extract, and Export one user data dir at a time, so at most one dir's copies are on disk. Its
// error means the run could not finish, such as a failed output write; every other failure is a Result.
func Execute(ctx context.Context, p Plan, req Request) (Summary, error) {
	keys := dumpKeys(req.Keys)
	if req.Keys == nil {
		keys = machineKeys(loginKeychain(req))
	}
	return execute(ctx, p, req, extractorsOf, keys)
}

func execute(ctx context.Context, p Plan, req Request, extractors extractorsFunc, keys keySource) (Summary, error) {
	w, err := export.NewWriter(req.Dir, req.Format)
	if err != nil {
		return Summary{}, err
	}
	results := append([]Result(nil), p.Skipped...)
	for _, dir := range p.Dirs {
		dirResults, err := executeDir(ctx, dir, p, w, extractors, keys)
		results = append(results, dirResults...)
		if err != nil {
			return Summary{Results: results}, errors.Join(err, w.Close())
		}
	}
	return Summary{Results: results}, w.Close()
}

func executeDir(ctx context.Context, dir browser.UserDataDir, p Plan, w *export.Writer, extractors extractorsFunc,
	keys keySource,
) ([]Result, error) {
	extractions := extractionsOf(p, dir.ID)
	if len(extractions) == 0 {
		return nil, nil
	}
	log.Infof("Extracting %s...", dir.Def.Name)

	d, err := snapshot.Temp()
	if err != nil {
		return failAll(extractions, err), nil
	}
	defer closeSnapshot(d)
	if err := d.Capture(ctx, filesOf(p, dir.ID)); err != nil {
		if ctx.Err() != nil {
			return nil, err
		}
		return failAll(extractions, err), nil
	}
	snap := d.Snapshot(string(dir.ID))

	var (
		dk         *dirKeys
		masterKeys func(context.Context) masterkey.Set
	)
	if dir.Def.Base == browser.Chromium {
		dk = &dirKeys{retrieve: func(ctx context.Context) (masterkey.Set, error) { return keys(ctx, dir, snap) }}
		masterKeys = dk.get
	}

	profiles := make(map[string]browser.Profile, len(dir.Profiles))
	byProfile := make(map[string]browser.Extractors, len(dir.Profiles))
	for _, prof := range dir.Profiles {
		profiles[prof.Name] = prof
		byProfile[prof.Name] = extractors(dir, prof)
	}

	results := make([]Result, 0, len(extractions)+1)
	for _, e := range extractions {
		x, ok := byProfile[e.Profile][e.Category]
		if !ok || x.Extract == nil {
			results = append(results, Result{Extraction: e, Outcome: OutcomeUnsupported})
			continue
		}
		prof := profiles[e.Profile]
		c := &collector{browser: dir.Def.Name, profile: prof.Name}
		err := x.Extract(ctx, browser.NewProfileSnapshot(snap, prof, masterKeys), c)
		// Records emitted before an extraction failed are kept, as they always were.
		if writeErr := w.Write(e.Category, c.records); writeErr != nil {
			return results, writeErr
		}
		results = append(results, c.result(e, err))
	}
	if dk != nil && dk.err != nil {
		results = append(results, dirFailure(dir.ID, fmt.Errorf("master key retrieval: %w", dk.err)))
	}
	return results, nil
}

func extractionsOf(p Plan, id browser.DirID) []Extraction {
	var out []Extraction
	for _, e := range p.Extractions {
		if e.DirID == id {
			out = append(out, e)
		}
	}
	return out
}

func filesOf(p Plan, id browser.DirID) []snapshot.File {
	var out []snapshot.File
	for _, f := range p.Files {
		if f.DirID == string(id) {
			out = append(out, f)
		}
	}
	return out
}

func failAll(extractions []Extraction, err error) []Result {
	results := make([]Result, len(extractions))
	for i, e := range extractions {
		results[i] = Result{Extraction: e, Outcome: OutcomeError, Err: err}
	}
	return results
}

func closeSnapshot(d *snapshot.Dir) {
	if err := d.Close(); err != nil {
		log.Warnf("%v", err)
	}
}

// collector stamps every record with its browser and profile, and keeps one extraction's output for the writer.
type collector struct {
	browser, profile string
	records          []record.Record
	failed           int
	firstFail        error
}

func (c *collector) Add(r record.Record) {
	record.Stamp(r, c.browser, c.profile)
	c.records = append(c.records, r)
}

func (c *collector) Fail(err error) {
	c.failed++
	if c.firstFail == nil {
		c.firstFail = err
	}
	log.Debugf("%s/%s: %v", c.browser, c.profile, err)
}

// result applies the outcome rules: a source that vanished after planning is absent, and problems with no record to
// show for them are an error, while problems beside records are partial.
func (c *collector) result(e Extraction, err error) Result {
	r := Result{Extraction: e, Count: len(c.records), Failed: c.failed}
	switch {
	case err != nil && errors.Is(err, fs.ErrNotExist) && r.Count == 0:
		r.Outcome = OutcomeAbsent
	case err != nil:
		r.Outcome, r.Err = OutcomeError, err
	case r.Failed > 0 && r.Count == 0:
		r.Outcome, r.Err = OutcomeError, c.firstFail
	case r.Failed > 0:
		r.Outcome = OutcomePartial
	default:
		r.Outcome = OutcomeOK
	}
	return r
}

// dirKeys retrieves a user data dir's keys on first use and keeps them, and the error, for the dir's other profiles.
// Go 1.20 has no sync.OnceValues.
type dirKeys struct {
	once     sync.Once
	retrieve func(context.Context) (masterkey.Set, error)
	keys     masterkey.Set
	err      error
}

func (k *dirKeys) get(ctx context.Context) masterkey.Set {
	k.once.Do(func() { k.keys, k.err = k.retrieve(ctx) })
	return k.keys
}

// machineKeys asks this machine's secret stores; on macOS that includes the run's login keychain.
func machineKeys(kc masterkey.LoginKeychain) keySource {
	return func(ctx context.Context, dir browser.UserDataDir, snap snapshot.Snapshot) (masterkey.Set, error) {
		loc, err := chromium.KeyLocator(dir, snap)
		if err != nil {
			return nil, err
		}
		loc.Keychain = kc
		return masterkey.RetrieveAll(ctx, loc)
	}
}

func dumpKeys(keys map[browser.DirID]masterkey.Set) keySource {
	return func(_ context.Context, dir browser.UserDataDir, _ snapshot.Snapshot) (masterkey.Set, error) {
		return keys[dir.ID], nil
	}
}
