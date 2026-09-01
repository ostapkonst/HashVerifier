// Package verify implements the `hashverifier verify` subcommand.
package verify

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
	"github.com/ostapkonst/HashVerifier/internal/domain/algorithm"
	resultpkg "github.com/ostapkonst/HashVerifier/internal/domain/result"
	"github.com/ostapkonst/HashVerifier/internal/platform/errs"
	serviceverify "github.com/ostapkonst/HashVerifier/internal/service/verify"
)

func runVerify(cmd *cobra.Command, args []string) error {
	algo, err := base.FlagString(cmd, "algorithm")
	if err != nil {
		return &base.ExitError{Code: 1, Err: err}
	}

	return base.RunWithShutdown(cmd, func(ctx context.Context) error {
		return execVerify(ctx, cmd, args, algo)
	})
}

func execVerify(ctx context.Context, cmd *cobra.Command, args []string, algorithmFlag string) error {
	checksumFile := filepath.Clean(args[0])

	algo, err := algorithm.ResolveAlgorithm(algorithmFlag, checksumFile)
	if err != nil {
		return &base.ExitError{Code: 1, Err: fmt.Errorf("unsupported algorithm: %w", err)}
	}

	cfg := serviceverify.VerifyConfig{
		ChecksumFile: checksumFile,
		Algorithm:    algo,
		OnFileVerified: func(res resultpkg.VerifyResult) {
			commonFields := func(event *zerolog.Event, err error) *zerolog.Event {
				logger := event.
					Str("file", res.Path).
					Str("status", res.Status.String()).
					Str("size", bytesize.New(float64(res.ReadBytes)).String()).
					Str("expected_hash", res.ExpectedHash).
					Str("actual_hash", res.ActualHash)

				if err != nil {
					logger = logger.Err(err)
				}

				return logger
			}

			if res.Status == resultpkg.HashMismatch {
				commonFields(log.Warn(), res.Err).Msg("Mismatch")
				return
			}

			if res.Status == resultpkg.Unreadable {
				commonFields(log.Error(), res.Err).Msg("Unreadable")
				return
			}

			commonFields(log.Info(), nil).Msg("Matched")
		},
	}

	log.Info().
		Str("checksum_file", checksumFile).
		Str("algorithm", algo.String()).
		Msg("Starting verification")

	res, err := serviceverify.VerifyChecksums(ctx, cfg)
	if err != nil {
		// Cancellation is a user abort only when it is the chain's sole cause; a joined failure must surface
		// as an error (exit 1) instead of being masked as a cancel (exit 130).
		if errs.IsContextDone(err) {
			log.Warn().Msg("Verification canceled")
			return &base.ExitError{Code: 130, Err: context.Canceled, Silent: true}
		}

		return &base.ExitError{Code: 1, Err: fmt.Errorf("failed to verify checksums: %w", err)}
	}

	stats := res.Stats
	log.Info().
		Int("matched", stats.Matched).
		Int("mismatch", stats.Mismatch).
		Int("unreadable", stats.Unreadable).
		Int("pending", stats.Pending()).
		Int("total_files", stats.TotalFiles).
		Msg("Verification stats")

	log.Info().Msg("Verification completed")

	if stats.Mismatch > 0 || stats.Unreadable > 0 {
		return &base.ExitError{
			Code: 1,
			Err:  fmt.Errorf("%d mismatch, %d unreadable of %d files", stats.Mismatch, stats.Unreadable, stats.TotalFiles),
		}
	}

	return nil
}

// NewCmd assembles the verify subcommand and its flags for the root command.
func NewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "verify <checksum_file>",
		Short: "Verify files against checksum file",
		Long: strings.Trim(dedent.Dedent(`
			Verify files against checksum file.
			Algorithm is determined in this order: --algorithm flag, SUMS-style filename, checksum file extension.
			The --algorithm flag requires a leading dot (e.g., ".sha256").

			Supported algorithms: .sfv (CRC32), .md4, .md5, .sha1, .sha256, .sha384, .sha512, .sha3-256, .sha3-384, .sha3-512, .blake3, .xxh3, .xxh128.`,
		), "\n"),
		Args: cobra.ExactArgs(1),
		RunE: runVerify,
	}

	cmd.Flags().String("algorithm", "", "Hash algorithm (e.g., .sha256, .md5, .sfv); if not set, determined from checksum file extension")

	return cmd
}
