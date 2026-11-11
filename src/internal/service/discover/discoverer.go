// Package discover contains the use-case orchestration for scanning sources for files whose names
// embed a CRC-32, comparing it with the checksum computed from the file contents, and optionally
// adding, fixing, or removing that checksum.
package discover

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/rs/zerolog/log"

	"github.com/ostapkonst/HashVerifier/internal/domain/algorithm"
	"github.com/ostapkonst/HashVerifier/internal/domain/embedded"
	"github.com/ostapkonst/HashVerifier/internal/domain/hashfn"
	"github.com/ostapkonst/HashVerifier/internal/domain/hidden"
	"github.com/ostapkonst/HashVerifier/internal/domain/result"
	"github.com/ostapkonst/HashVerifier/internal/domain/scan"
	"github.com/ostapkonst/HashVerifier/internal/domain/walk"
	"github.com/ostapkonst/HashVerifier/internal/platform/crash"
	"github.com/ostapkonst/HashVerifier/internal/platform/errs"
	"github.com/ostapkonst/HashVerifier/internal/platform/fs"
)

// zeroCRC32 is the placeholder shown when a file has no checksum or was not hashed.
var zeroCRC32 = strings.Repeat("0", algorithm.GetHashLength(algorithm.CRC32))

// Sentinel skip reasons rendered in the discover table's Note column.
var (
	errNoEmbeddedChecksum   = errors.New("no embedded checksum")
	errChecksumOutsideScope = errors.New("embedded checksum outside the scan scope")
)

// DiscovererStatusType marks whether a Discoverer has begun or completed its run.
type DiscovererStatusType int

// DiscovererStatusType values: Finished is both the initial and terminal state, Started means a run is in flight.
const (
	DiscovererStatusFinished DiscovererStatusType = iota
	DiscovererStatusStarted
)

// Discoverer walks the configured sources, classifies each file against the selected scope and
// action, and hashes or renames it as needed.
// Stats and MarkDiscovered are mutex-protected; Wait/Results read channels installed by Start
// and are safe to call from any goroutine once Start has returned.
//
// A Discoverer instance is intended for a single run: the internal ctx is canceled when the run
// completes, so callers needing multiple runs should construct a fresh Discoverer per call.
type Discoverer struct {
	rwm    sync.RWMutex
	ctx    context.Context
	cancel context.CancelFunc

	sources             []string
	followSymbolicLinks bool
	skipHidden          bool
	filesToScan         scan.FilesToScan
	action              scan.Action

	stats                   result.DiscovererStats
	currFileHashingProgress atomic.Value
	speedTracker            *result.SpeedTracker

	status DiscovererStatusType

	err      chan error
	resultCh chan result.DiscoverResult
	done     chan struct{}
}

// NewDiscoverer constructs a Discoverer; Start kicks off the walk-and-hash run, Wait/Results consume it.
func NewDiscoverer(
	ctx context.Context,
	sources []string,
	followSymbolicLinks, skipHidden bool,
	filesToScan scan.FilesToScan,
	action scan.Action,
) *Discoverer {
	ctx, cancel := context.WithCancel(ctx)

	d := &Discoverer{
		ctx:                 ctx,
		cancel:              cancel,
		sources:             sources,
		followSymbolicLinks: followSymbolicLinks,
		skipHidden:          skipHidden,
		filesToScan:         filesToScan,
		action:              action,
		status:              DiscovererStatusFinished,
		speedTracker:        result.NewSpeedTracker(),
	}

	return d
}

// discoveredFile is one file found under the configured sources; Checksum is empty when the name
// carries no embedded CRC-32.
type discoveredFile struct {
	Path        string
	Checksum    string
	HasChecksum bool
	// Hidden marks the file as skipped without hashing: every source that found it considers
	// it hidden.
	Hidden bool
}

// Start launches the walk-and-hash goroutine; a no-op while a run is in flight.
// The internal ctx is canceled when the run ends, successful or failed, so a second Start
// on the same instance re-launches a goroutine that fails immediately with ctx.Err(). Callers
// needing multiple runs should construct a fresh Discoverer per run.
func (d *Discoverer) Start() {
	d.rwm.Lock()
	defer d.rwm.Unlock()

	if d.status != DiscovererStatusFinished {
		return
	}

	d.resultCh = make(chan result.DiscoverResult, 1)
	d.done = make(chan struct{})
	d.err = make(chan error, 1)

	d.status = DiscovererStatusStarted

	d.stats = result.NewDiscovererStats()
	d.currFileHashingProgress.Store(func() float64 { return 0 })
	d.speedTracker.Reset()

	crash.Go("discover.run", d.run)
}

// Wait blocks until the current run completes and returns the terminal error (or nil on success).
func (d *Discoverer) Wait() error {
	<-d.done
	return <-d.err
}

// Stats returns the current aggregate progress snapshot including file-hash progress and rolling speed.
func (d *Discoverer) Stats() result.DiscovererStats {
	fileHashProgress := d.currFileHashingProgress.Load().(func() float64)

	d.rwm.RLock()
	defer d.rwm.RUnlock()

	stats := d.stats
	stats.FileHashingProgress = fileHashProgress()
	stats.Speed = d.speedTracker.Speed()

	return stats
}

// Results returns the receive-only channel of per-file DiscoverResult events for the current run;
// it is closed when the run finishes.
func (d *Discoverer) Results() <-chan result.DiscoverResult {
	return d.resultCh
}

// MarkDiscovered classifies status into the Success / Mismatch / Failed / Skipped counters;
// called by the consumer after each per-file result. Panics on a status it does not recognize.
func (d *Discoverer) MarkDiscovered(status result.DiscoverStatusType) {
	d.rwm.Lock()
	defer d.rwm.Unlock()

	switch status {
	case result.DiscoverSuccess:
		d.stats.Success++
	case result.DiscoverMismatch:
		d.stats.Mismatch++
	case result.DiscoverFailed:
		d.stats.Failed++
	case result.DiscoverSkipped:
		d.stats.Skipped++
	default:
		panic("unknown discovery status")
	}
}

func (d *Discoverer) updateCurrentFileOrStatus(file string) {
	d.rwm.Lock()
	defer d.rwm.Unlock()

	d.stats.CurrentFileOrStatus = file
}

func (d *Discoverer) updateStatsPending(totalFiles int) {
	d.rwm.Lock()
	defer d.rwm.Unlock()

	d.stats.TotalFiles = totalFiles
}

func (d *Discoverer) run() {
	defer func() {
		d.rwm.Lock()
		defer d.rwm.Unlock()

		d.status = DiscovererStatusFinished
	}()

	defer close(d.done)
	defer func() {
		// Zero the speed before consumers can read the terminal snapshot: speed is a live-run metric.
		d.updateCurrentFileOrStatus(result.StatusReady)
		d.speedTracker.Reset()
	}()
	defer close(d.err)
	defer close(d.resultCh)
	defer d.cancel()
	// Every early return must leave a terminal error in d.err (cap-1 channel, single-producer);
	// cancel exits send ctx.Err() themselves because they bypass the err-send branch.

	d.updateCurrentFileOrStatus("forming a list of files for discovery...")

	entries, err := d.collectEntries()
	if err != nil {
		d.err <- fmt.Errorf("discovering sources: %w", err)
		return
	}

	d.updateStatsPending(len(entries))

	for _, entry := range entries {
		select {
		case <-d.ctx.Done():
			d.err <- fmt.Errorf("discovering sources: %w", d.ctx.Err())
			return
		default:
		}

		d.updateCurrentFileOrStatus(entry.Path)
		d.currFileHashingProgress.Store(func() float64 { return 0 })

		if entry.Hidden {
			if !d.emit(skippedResult(entry, hidden.ErrHidden)) {
				d.err <- fmt.Errorf("discovering sources: %w", d.ctx.Err())
				return
			}

			continue
		}

		if inScope, reason := d.scopeDecision(entry); !inScope {
			if !d.emit(skippedResult(entry, reason)) {
				d.err <- fmt.Errorf("discovering sources: %w", d.ctx.Err())
				return
			}

			continue
		}

		res, ok := d.processEntry(entry)
		if !ok {
			d.err <- fmt.Errorf("discovering sources: %w", d.ctx.Err())
			return
		}

		if !d.emit(res) {
			d.err <- fmt.Errorf("discovering sources: %w", d.ctx.Err())
			return
		}
	}
}

// scopeDecision reports whether entry is selected by files-to-scan; reason explains a skip.
func (d *Discoverer) scopeDecision(entry discoveredFile) (bool, error) {
	switch d.filesToScan {
	case scan.FilesToScanAll:
		return true, nil
	case scan.FilesToScanWithChecksum:
		if entry.HasChecksum {
			return true, nil
		}

		return false, errNoEmbeddedChecksum
	case scan.FilesToScanWithoutChecksum:
		if !entry.HasChecksum {
			return true, nil
		}

		return false, errChecksumOutsideScope
	default:
		return false, fmt.Errorf("unknown files to scan: %s", d.filesToScan)
	}
}

// processEntry applies the selected action to an in-scope entry; ok=false means ctx was canceled.
func (d *Discoverer) processEntry(entry discoveredFile) (result.DiscoverResult, bool) {
	switch d.action {
	case scan.ActionReportOnly:
		if !entry.HasChecksum {
			return skippedResult(entry, errNoEmbeddedChecksum), true
		}

		return d.verifyEntry(entry)
	case scan.ActionAddMissing:
		if !entry.HasChecksum {
			return d.addEntry(entry)
		}

		res, ok := d.verifyEntry(entry)
		if ok && res.Status == result.DiscoverMismatch {
			res.Note = "mismatch not fixed (Add missing)"
		}

		return res, ok
	case scan.ActionAddOrFix:
		if !entry.HasChecksum {
			return d.addEntry(entry)
		}

		return d.fixEntry(entry)
	case scan.ActionRemove:
		if !entry.HasChecksum {
			return skippedResult(entry, errNoEmbeddedChecksum), true
		}

		return d.removeEntry(entry)
	default:
		return failedResult(entry.Path, embeddedHashOrZero(entry), zeroCRC32, 0,
			fmt.Errorf("unknown action: %s", d.action)), true
	}
}

// verifyEntry hashes entry and compares the digest with the checksum embedded in its name.
func (d *Discoverer) verifyEntry(entry discoveredFile) (result.DiscoverResult, bool) {
	hashResult, err := d.hashFile(entry.Path)
	if err != nil {
		if errs.IsContextDone(err) {
			return result.DiscoverResult{}, false
		}

		return failedResult(entry.Path, entry.Checksum, strings.ToLower(hashResult.Hash), hashResult.ReadBytes, err), true
	}

	actual := strings.ToLower(hashResult.Hash)
	if strings.EqualFold(actual, entry.Checksum) {
		return successResult(entry.Path, entry.Checksum, actual, hashResult.ReadBytes, ""), true
	}

	return mismatchResult(entry.Path, entry.Checksum, actual, hashResult.ReadBytes), true
}

// addEntry hashes a checksum-less file and inserts the computed checksum before its extension.
func (d *Discoverer) addEntry(entry discoveredFile) (result.DiscoverResult, bool) {
	hashResult, err := d.hashFile(entry.Path)
	if err != nil {
		if errs.IsContextDone(err) {
			return result.DiscoverResult{}, false
		}

		return failedResult(entry.Path, embeddedHashOrZero(entry), strings.ToLower(hashResult.Hash), hashResult.ReadBytes, err), true
	}

	actual := strings.ToLower(hashResult.Hash)
	newBase := embedded.InsertChecksum(filepath.Base(entry.Path), actual)

	newPath, err := renameChecked(entry.Path, newBase)
	if err != nil {
		return failedResult(entry.Path, embeddedHashOrZero(entry), actual, hashResult.ReadBytes, err), true
	}

	return successResult(newPath, actual, actual, hashResult.ReadBytes, "added ["+actual+"]"), true
}

// fixEntry hashes entry and replaces the embedded checksum when it does not match the content.
func (d *Discoverer) fixEntry(entry discoveredFile) (result.DiscoverResult, bool) {
	hashResult, err := d.hashFile(entry.Path)
	if err != nil {
		if errs.IsContextDone(err) {
			return result.DiscoverResult{}, false
		}

		return failedResult(entry.Path, entry.Checksum, strings.ToLower(hashResult.Hash), hashResult.ReadBytes, err), true
	}

	actual := strings.ToLower(hashResult.Hash)
	if strings.EqualFold(actual, entry.Checksum) {
		return successResult(entry.Path, entry.Checksum, actual, hashResult.ReadBytes, ""), true
	}

	newBase, ok := embedded.ReplaceChecksum(filepath.Base(entry.Path), actual)
	if !ok {
		return failedResult(entry.Path, entry.Checksum, actual, hashResult.ReadBytes,
			fmt.Errorf("replace checksum in %s: checksum disappeared", entry.Path)), true
	}

	newPath, err := renameChecked(entry.Path, newBase)
	if err != nil {
		return failedResult(entry.Path, entry.Checksum, actual, hashResult.ReadBytes, err), true
	}

	return successResult(newPath, actual, actual, hashResult.ReadBytes,
		"fixed ["+entry.Checksum+"] → ["+actual+"]"), true
}

// removeEntry strips the embedded checksum without reading the file, after a cheap regular-file check.
func (d *Discoverer) removeEntry(entry discoveredFile) (result.DiscoverResult, bool) {
	if err := ensureRegularFile(entry.Path); err != nil {
		return failedResult(entry.Path, entry.Checksum, zeroCRC32, 0, err), true
	}

	newBase, ok := embedded.RemoveChecksum(filepath.Base(entry.Path))
	if !ok {
		return failedResult(entry.Path, entry.Checksum, zeroCRC32, 0,
			fmt.Errorf("remove checksum from %s: checksum disappeared", entry.Path)), true
	}

	newPath, err := renameChecked(entry.Path, newBase)
	if err != nil {
		return failedResult(entry.Path, entry.Checksum, zeroCRC32, 0, err), true
	}

	return successResult(newPath, zeroCRC32, zeroCRC32, 0, "removed ["+entry.Checksum+"]"), true
}

// hashFile streams entry through CRC-32 and wires its progress into the shared stats.
func (d *Discoverer) hashFile(path string) (hashfn.HashResult, error) {
	hashCalc := hashfn.NewHashCalculator(path, algorithm.CRC32, d.speedTracker)
	d.currFileHashingProgress.Store(hashCalc.Progress)

	return hashCalc.Calculate(d.ctx)
}

// ensureRegularFile refuses non-regular paths before a rename, mirroring the hashing guard.
func ensureRegularFile(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		// fs.PathError already embeds the op ("stat") and path; another prefix would double them.
		return err
	}

	if !info.Mode().IsRegular() {
		return fmt.Errorf("%s: %w", path, hashfn.ErrNotARegularFile)
	}

	return nil
}

// renameChecked renames path to newBase in the same directory, refusing an existing target so a
// checksum rewrite never overwrites another file.
func renameChecked(path, newBase string) (string, error) {
	target := filepath.Join(filepath.Dir(path), newBase)
	if target == path {
		return path, nil
	}

	if _, err := os.Lstat(target); err == nil {
		return "", fmt.Errorf("target already exists: %s", target)
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", fmt.Errorf("stat target %s: %w", target, err)
	}

	if err := os.Rename(path, target); err != nil {
		return "", fmt.Errorf("rename %s to %s: %w", path, target, err)
	}

	return target, nil
}

func skippedResult(entry discoveredFile, cause error) result.DiscoverResult {
	return result.DiscoverResult{
		Path:         entry.Path,
		EmbeddedHash: embeddedHashOrZero(entry),
		ActualHash:   zeroCRC32,
		Status:       result.DiscoverSkipped,
		Err:          cause,
	}
}

func successResult(path, expected, actual string, readBytes int64, note string) result.DiscoverResult {
	return result.DiscoverResult{
		Path:         path,
		EmbeddedHash: expected,
		ActualHash:   actual,
		ReadBytes:    readBytes,
		Status:       result.DiscoverSuccess,
		Note:         note,
	}
}

func mismatchResult(path, expected, actual string, readBytes int64) result.DiscoverResult {
	return result.DiscoverResult{
		Path:         path,
		EmbeddedHash: expected,
		ActualHash:   actual,
		ReadBytes:    readBytes,
		Status:       result.DiscoverMismatch,
	}
}

func failedResult(path, expected, actual string, readBytes int64, err error) result.DiscoverResult {
	return result.DiscoverResult{
		Path:         path,
		EmbeddedHash: expected,
		ActualHash:   actual,
		ReadBytes:    readBytes,
		Status:       result.DiscoverFailed,
		Err:          err,
	}
}

func embeddedHashOrZero(entry discoveredFile) string {
	if entry.HasChecksum {
		return entry.Checksum
	}

	return zeroCRC32
}

// emit sends res to the result channel, returning false when the context is canceled first.
func (d *Discoverer) emit(res result.DiscoverResult) bool {
	select {
	case d.resultCh <- res:
		return true
	case <-d.ctx.Done():
		return false
	}
}

// collectEntries resolves the configured sources into deduplicated files, recording the embedded
// checksum when present and classifying hidden files when skip-hidden is on; a directory source is
// walked per the follow-symbolic-links option, a non-directory source is taken as-is. Deduplication
// is by path key (cleaned, and case-folded on Windows/macOS), so overlapping sources are listed
// once; symlink aliases of the same file are not merged. Entries the walk cannot enumerate are
// logged and ignored, matching generate.
//
// A file found by several sources is skipped as hidden only when every one of them considers
// it hidden: a source listed explicitly is not governed by skip-hidden and wins over a broader
// source that hides the same file, so the outcome never depends on the order of the sources.
func (d *Discoverer) collectEntries() (entries []discoveredFile, err error) {
	index := make(fs.PathIndex)

	record := func(entry discoveredFile, hidden bool) {
		idx, isNew := index.Index(entry.Path)
		if isNew {
			entry.Hidden = hidden
			entries = append(entries, entry)

			return
		}

		entries[idx].Hidden = entries[idx].Hidden && hidden
	}

	for _, source := range d.sources {
		src, aerr := filepath.Abs(source)
		if aerr != nil {
			return nil, aerr
		}

		info, serr := os.Stat(src)
		if serr != nil {
			return nil, serr
		}

		if info.IsDir() {
			wr, werr := walk.WalkDir(d.ctx, src, d.followSymbolicLinks, false)
			if werr != nil {
				return nil, werr
			}

			var checker *fs.HiddenChecker
			if d.skipHidden {
				checker = fs.NewHiddenChecker(src)
			}

			for _, f := range wr.Files {
				entry := checksumEntry(f)
				hidden := false

				if checker != nil {
					rel, rerr := filepath.Rel(src, f)
					if rerr != nil {
						return nil, fmt.Errorf("failed to calculate relative path: %w", rerr)
					}

					hidden = checker.IsHidden(rel)
				}

				record(entry, hidden)
			}

			for _, s := range wr.Skipped {
				log.Warn().Err(s.Err).Str("path", s.Path).Msg("walk: skipped entry")
			}

			continue
		}

		record(checksumEntry(src), false)
	}

	return entries, nil
}

// checksumEntry builds a discoveredFile for path, recording the embedded CRC-32 when present.
func checksumEntry(path string) discoveredFile {
	checksum, ok := embedded.Checksum(filepath.Base(path))
	if !ok {
		return discoveredFile{Path: path}
	}

	return discoveredFile{
		Path:        path,
		Checksum:    strings.ToLower(checksum),
		HasChecksum: true,
	}
}
