// Package discover implements the `hashverifier discover` subcommand.
package discover

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/inhies/go-bytesize"
	"github.com/lithammer/dedent"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	"github.com/ostapkonst/HashVerifier/internal/adapter/cli/base"
	"github.com/ostapkonst/HashVerifier/internal/domain/embedded"
	resultpkg "github.com/ostapkonst/HashVerifier/internal/domain/result"
	"github.com/ostapkonst/HashVerifier/internal/platform/errs"
	servicediscover "github.com/ostapkonst/HashVerifier/internal/service/discover"
)

func runDiscover(cmd *cobra.Command, args []string) error {
	return base.RunWithShutdown(cmd, func(ctx context.Context) error {
		return execDiscover(ctx, cmd, args)
	})
}

func execDiscover(ctx context.Context, cmd *cobra.Command, args []string) error {
	sources := make([]string, 0, len(args))
	for _, arg := range args {
		sources = append(sources, filepath.Clean(arg))
	}

	cfgSettings, err := base.LoadAndLog(cmd)
	if err != nil {
		return &base.ExitError{Code: 1, Err: err}
	}

	filterStr, err := base.FlagStringOrDefault(cmd, "filter", cfgSettings.Discover.SourcesDialog.Filter)
	if err != nil {
		return &base.ExitError{Code: 1, Err: err}
	}

	filter, ok := embedded.FilterModeFromString(filterStr)
	if !ok {
		return &base.ExitError{
			Code: 1,
			Err:  fmt.Errorf("invalid --filter %q: expected all, with_checksum, or without_checksum", filterStr),
		}
	}

	renameFlag, err := base.FlagString(cmd, "rename")
	if err != nil {
		return &base.ExitError{Code: 1, Err: err}
	}

	var renameMode embedded.RenameMode

	if renameFlag != "" {
		mode, modeOK := embedded.RenameModeFromString(renameFlag)
		if !modeOK {
			return &base.ExitError{
				Code: 1,
				Err:  fmt.Errorf("invalid --rename %q: expected add, remove, or update", renameFlag),
			}
		}

		renameMode = mode
	}

	dryRun, err := base.FlagBool(cmd, "dry-run")
	if err != nil {
		return &base.ExitError{Code: 1, Err: err}
	}

	if dryRun && renameFlag == "" {
		log.Warn().Msg("--dry-run has no effect without --rename")
	}

	var discovered []resultpkg.DiscoverResult

	cfg := servicediscover.DiscoverConfig{
		Sources: sources,
		Filter:  filter,
		OnFileDiscovered: func(res resultpkg.DiscoverResult) {
			// Collected results are only replayed by the rename plan; without --rename they
			// would just accumulate in memory for nothing.
			if renameFlag != "" {
				discovered = append(discovered, res)
			}

			commonFields := func(event *zerolog.Event, err error) *zerolog.Event {
				logger := event.
					Str("file", res.Path).
					Str("status", res.Status.String()).
					Str("size", bytesize.New(float64(res.ReadBytes)).String()).
					Str("expected_hash", res.EmbeddedHash).
					Str("actual_hash", res.ActualHash)

				if err != nil {
					logger = logger.Err(err)
				}

				return logger
			}

			switch res.Status {
			case resultpkg.DiscoverMismatch:
				commonFields(log.Warn(), res.Err).Msg("Mismatch")
			case resultpkg.DiscoverFailed:
				commonFields(log.Error(), res.Err).Msg("Failed")
			case resultpkg.DiscoverNoChecksum:
				commonFields(log.Info(), nil).Msg("No checksum")
			default:
				commonFields(log.Info(), nil).Msg("Matched")
			}
		},
	}

	log.Info().
		Strs("sources", cfg.Sources).
		Str("filter", filter.String()).
		Msg("Starting discovery")

	res, err := servicediscover.DiscoverFiles(ctx, cfg)
	if err != nil {
		// Cancellation is a user abort only when it is the chain's sole cause; a joined failure must surface
		// as an error (exit 1) instead of being masked as a cancel (exit 130).
		if errs.IsContextDone(err) {
			log.Warn().Msg("Discovery canceled")
			return &base.ExitError{Code: 130, Err: context.Canceled, Silent: true}
		}

		return &base.ExitError{Code: 1, Err: fmt.Errorf("failed to discover files: %w", err)}
	}

	stats := res.Stats

	log.Info().
		Int("matched", stats.Matched).
		Int("mismatch", stats.Mismatch).
		Int("failed", stats.Failed).
		Int("no_checksum", stats.NoChecksum).
		Int("pending", stats.Pending()).
		Int("total_files", stats.TotalFiles).
		Msg("Discovery stats")
	log.Info().Msg("Discovery completed")

	var runErrs []error

	if stats.Mismatch > 0 || stats.Failed > 0 {
		runErrs = append(runErrs, fmt.Errorf(
			"%d mismatch, %d failed of %d files", stats.Mismatch, stats.Failed, stats.TotalFiles,
		))
	}

	if renameFlag != "" {
		if err := applyRenamePlan(ctx, discovered, renameMode, dryRun); err != nil {
			if errs.IsContextDone(err) {
				log.Warn().Msg("Rename canceled")
				return &base.ExitError{Code: 130, Err: context.Canceled, Silent: true}
			}

			runErrs = append(runErrs, err)
		}
	}

	if err := errors.Join(runErrs...); err != nil {
		return &base.ExitError{Code: 1, Err: err}
	}

	return nil
}

func applyRenamePlan(ctx context.Context, discovered []resultpkg.DiscoverResult, mode embedded.RenameMode, dryRun bool) error {
	actions := make([]embedded.RenameAction, 0)

	for _, res := range discovered {
		// Failed files have no computed checksum, so they cannot be renamed.
		if res.Status == resultpkg.DiscoverFailed {
			continue
		}

		newName, changed := embedded.PlanRename(filepath.Base(res.Path), res.ActualHash, mode)
		if !changed {
			continue
		}

		actions = append(actions, embedded.RenameAction{
			From: res.Path,
			To:   filepath.Join(filepath.Dir(res.Path), newName),
		})
	}

	log.Info().
		Int("files", len(actions)).
		Str("mode", mode.String()).
		Bool("dry_run", dryRun).
		Msg("Starting rename")

	if dryRun {
		for _, action := range actions {
			log.Info().
				Str("from", action.From).
				Str("to", action.To).
				Msg("Would rename")
		}

		log.Info().
			Int("renamed", 0).
			Int("failed", 0).
			Int("total", len(actions)).
			Bool("dry_run", true).
			Msg("Rename stats")
		log.Info().Msg("Rename completed")

		return nil
	}

	stats, err := servicediscover.RenameFiles(ctx, actions, func(outcome servicediscover.RenameOutcome) {
		if outcome.Err != nil {
			log.Error().
				Err(outcome.Err).
				Str("from", outcome.Action.From).
				Str("to", outcome.Action.To).
				Msg("Failed to rename file")

			return
		}

		log.Info().
			Str("from", outcome.Action.From).
			Str("to", outcome.Action.To).
			Msg("Renamed")
	})
	if err != nil {
		return err
	}

	log.Info().
		Int("renamed", stats.Renamed).
		Int("failed", stats.Failed).
		Int("pending", stats.Pending()).
		Int("total", stats.TotalFiles).
		Msg("Rename stats")
	log.Info().Msg("Rename completed")

	if stats.Failed > 0 {
		return fmt.Errorf("%d of %d files failed to rename", stats.Failed, stats.TotalFiles)
	}

	return nil
}

// NewCmd assembles the discover subcommand and its flags for the root command.
func NewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "discover <source>...",
		Short: "Verify CRC-32 checksums embedded in file names",
		Long: strings.Trim(dedent.Dedent(`
			Scan files and directories for CRC-32 checksums embedded in file names
			(e.g. filename [1a2b3c4d].ext) and verify them against the file contents.
			Comparison is performed only when the name carries a checksum; files without one
			are reported as "No checksum" and do not affect the exit code. Symbolic links are
			followed; hidden files are scanned like any other file.

			The --filter setting selects which files are scanned: all, with_checksum,
			or without_checksum (default from discover.sources_dialog.filter).

			Use --rename add|remove|update to rewrite names from the computed checksums.
			Use --dry-run with --rename to log the planned renames without applying them.

			Exit code is 1 when any file mismatches or fails, or when a rename fails.`,
		), "\n"),
		Args: cobra.MinimumNArgs(1),
		RunE: runDiscover,
	}

	cmd.Flags().String("filter", "", "which files to scan: all, with_checksum, without_checksum (default from discover.sources_dialog.filter)")
	cmd.Flags().String("rename", "", "rename discovered files: add, remove, update (default: no rename)")
	cmd.Flags().Bool("dry-run", false, "with --rename, log the planned renames without applying them")

	return cmd
}
