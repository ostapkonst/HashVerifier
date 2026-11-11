// Package discover implements the `hashverifier discover` subcommand.
package discover

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/inhies/go-bytesize"
	"github.com/lithammer/dedent"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	"github.com/ostapkonst/HashVerifier/internal/adapter/cli/base"
	resultpkg "github.com/ostapkonst/HashVerifier/internal/domain/result"
	"github.com/ostapkonst/HashVerifier/internal/domain/scan"
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

	followSymbolicLinks, err := base.FlagBoolOrDefault(cmd, "follow-symbolic-links", cfgSettings.Discover.SourcesDialog.FollowSymbolicLinks)
	if err != nil {
		return &base.ExitError{Code: 1, Err: err}
	}

	skipHidden, err := base.FlagBoolOrDefault(cmd, "skip-hidden", cfgSettings.Discover.SourcesDialog.SkipHidden)
	if err != nil {
		return &base.ExitError{Code: 1, Err: err}
	}

	rawFilesToScan, err := base.FlagStringOrDefault(cmd, "files-to-scan", string(cfgSettings.Discover.SourcesDialog.FilesToScan))
	if err != nil {
		return &base.ExitError{Code: 1, Err: err}
	}

	filesToScan, err := scan.ParseFilesToScan(rawFilesToScan)
	if err != nil {
		return &base.ExitError{Code: 1, Err: err}
	}

	rawAction, err := base.FlagStringOrDefault(cmd, "action", string(cfgSettings.Discover.SourcesDialog.Action))
	if err != nil {
		return &base.ExitError{Code: 1, Err: err}
	}

	action, err := scan.ParseAction(rawAction)
	if err != nil {
		return &base.ExitError{Code: 1, Err: err}
	}

	cfg := servicediscover.DiscoverConfig{
		Sources:             sources,
		FollowSymbolicLinks: followSymbolicLinks,
		SkipHidden:          skipHidden,
		FilesToScan:         filesToScan,
		Action:              action,
		OnFileDiscovered: func(res resultpkg.DiscoverResult) {
			commonFields := func(event *zerolog.Event, err error) *zerolog.Event {
				logger := event.
					Str("file", res.Path).
					Str("status", res.Status.String()).
					Str("size", bytesize.New(float64(res.ReadBytes)).String()).
					Str("expected_hash", res.EmbeddedHash).
					Str("actual_hash", res.ActualHash)

				if res.Note != "" {
					logger = logger.Str("note", res.Note)
				}

				if err != nil {
					logger = logger.Err(err)
				}

				return logger
			}

			switch res.Status {
			case resultpkg.DiscoverSuccess:
				commonFields(log.Info(), nil).Msg("Success")
			case resultpkg.DiscoverMismatch:
				commonFields(log.Warn(), res.Err).Msg("Mismatch")
			case resultpkg.DiscoverFailed:
				commonFields(log.Error(), res.Err).Msg("Failed")
			case resultpkg.DiscoverSkipped:
				commonFields(log.Warn(), res.Err).Msg("Skipped")
			default:
				panic("unknown discovery status")
			}
		},
	}

	log.Info().
		Strs("sources", cfg.Sources).
		Bool("follow_symbolic_links", cfg.FollowSymbolicLinks).
		Bool("skip_hidden", cfg.SkipHidden).
		Str("files_to_scan", string(cfg.FilesToScan)).
		Str("action", string(cfg.Action)).
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
		Int("success", stats.Success).
		Int("mismatch", stats.Mismatch).
		Int("failed", stats.Failed).
		Int("skipped", stats.Skipped).
		Int("pending", stats.Pending()).
		Int("total_files", stats.TotalFiles).
		Msg("Discovery stats")
	log.Info().Msg("Discovery completed")

	if stats.Mismatch > 0 || stats.Failed > 0 {
		return &base.ExitError{
			Code: 1,
			Err: fmt.Errorf("%d mismatch, %d failed of %d files",
				stats.Mismatch, stats.Failed, stats.TotalFiles),
		}
	}

	return nil
}

// NewCmd assembles the discover subcommand and its flags for the root command.
func NewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "discover <source>...",
		Short: "Verify, add, fix, or remove CRC-32 checksums embedded in file names",
		Long: strings.Trim(dedent.Dedent(`
			Scan files and directories for CRC-32 checksums embedded in file names
			(e.g. filename [1a2b3c4d].ext). Every scanned file is listed: names that embed
			a checksum are verified, and the rest are reported as "Skipped".

			Use --files-to-scan to choose all files, only names that embed a checksum, or
			only names without one. Use --action to add missing checksums, add or fix
			mismatched ones, or remove checksums. The default action is report_only, which
			never renames files.

			Symbolic links are followed by default; pass --follow-symbolic-links=false to ignore
			them. Pass --skip-hidden to report hidden files and directories as "Skipped" instead
			of scanning them (dot-prefixed on Unix, hidden attribute on Windows).

			A source listed explicitly wins over a broader source that hides the same file, so the
			outcome does not depend on the order of the sources.

			A checksum mismatch, a file that could not be read or renamed, or an unavailable source
			is a failure; skipped entries are never failures.`,
		), "\n") + "\n\n" + base.ExitCodesHint,
		Args: cobra.MinimumNArgs(1),
		RunE: runDiscover,
	}

	cmd.Flags().Bool("follow-symbolic-links", false, "follow symbolic links when scanning directories (default from discover.sources_dialog.follow_symbolic_links)")
	cmd.Flags().Bool("skip-hidden", false, "skip hidden files and directories (default from discover.sources_dialog.skip_hidden)")
	cmd.Flags().String("files-to-scan", "", "files to scan: all, with_checksum, or without_checksum (default from discover.sources_dialog.files_to_scan)")
	cmd.Flags().String("action", "", "action: report_only, add_missing, add_or_fix, or remove (default from discover.sources_dialog.action)")

	return cmd
}
