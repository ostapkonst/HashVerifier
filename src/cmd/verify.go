package cmd

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/inhies/go-bytesize"
	"github.com/lithammer/dedent"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	"github.com/ostapkonst/HashVerifier/internal/action"
	"github.com/ostapkonst/HashVerifier/internal/checksum"
	"github.com/ostapkonst/HashVerifier/utils/gracer"
)

func runVerify(cmd *cobra.Command, args []string) error {
	ctx, cancel := context.WithCancel(cmd.Context())
	defer cancel()

	algorithmFlag, err := cmd.Flags().GetString("ext")
	if err != nil {
		return fmt.Errorf("internal error reading --ext flag: %w", err)
	}

	done := make(chan error, 1)

	gracer.AddCallback(func() error {
		cancel()
		return <-done
	})

	go func() {
		done <- execVerify(ctx, args, algorithmFlag)

		gracer.GracefulShutdown()
	}()

	return gracer.Wait()
}

func execVerify(ctx context.Context, args []string, algorithmFlag string) error {
	checksumFile := args[0]

	cfg := action.VerifyConfig{
		ChecksumFile: checksumFile,
		Extension:    algorithmFlag,
		OnFileVerified: func(res checksum.VerifyResult) {
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

			if res.Status == checksum.HashMismatch {
				commonFields(log.Warn(), res.Err).Msg("Mismatch")
				return
			}

			if res.Status == checksum.Unreadable {
				commonFields(log.Error(), res.Err).Msg("Unreadable")
				return
			}

			commonFields(log.Info(), nil).Msg("Matched")
		},
	}

	log.Info().
		Str("checksum_file", checksumFile).
		Msg("Starting verification")

	result, err := action.VerifyChecksums(ctx, cfg)
	if err != nil {
		if errors.Is(err, context.Canceled) {
			log.Warn().Msg("Verification canceled")
			return nil
		}

		return fmt.Errorf("failed to verify checksums: %w", err)
	}

	stats := result.Stats
	log.Info().
		Int("matched", stats.Matched).
		Int("mismatch", stats.Mismatch).
		Int("unreadable", stats.Unreadable).
		Int("pending", stats.Pending()).
		Int("total_files", stats.TotalFiles).
		Msg("Verification stats")

	log.Info().Msg("Verification completed")

	return nil
}

var verifyCmd = &cobra.Command{
	Use:   "verify <checksum_file>",
	Short: "Verify files against checksum file",
	Long: strings.Trim(dedent.Dedent(`
		Verify files against checksum file.
		Algorithm is determined automatically from file extension or can be set via --ext flag.

		Supported algorithms: .sfv (CRC32), .md4, .md5, .sha1, .sha256, .sha384, .sha512, .sha3-256, .sha3-384, .sha3-512, .blake3, .xxh3, .xxh128.`,
	), "\n"),
	Args: cobra.ExactArgs(1),
	RunE: runVerify,
}

func init() {
	verifyCmd.Flags().StringP("ext", "e", "", "Hash algorithm extension (e.g., .sha256, .md5, .sfv). If not set, determined from checksum file extension")
	rootCmd.AddCommand(verifyCmd)
}
