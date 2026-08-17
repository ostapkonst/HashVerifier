package cmd

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/lithammer/dedent"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	"github.com/ostapkonst/HashVerifier/internal/action"
	"github.com/ostapkonst/HashVerifier/utils/gracer"
)

func runHash(cmd *cobra.Command, args []string) error {
	ctx, cancel := context.WithCancel(cmd.Context())
	defer cancel()

	done := make(chan error, 1)

	gracer.AddCallback(func() error {
		cancel()
		return <-done
	})

	go func() {
		done <- execHash(ctx, cmd, args)

		gracer.GracefulShutdown()
	}()

	return gracer.Wait()
}

func execHash(ctx context.Context, cmd *cobra.Command, args []string) error {
	filePath := filepath.Clean(args[0])

	cfgSettings := loadAndLog(cmd)

	algorithms := flagStringSliceOrDefault(cmd, "algorithms", cfgSettings.Hash.Algorithms)
	for i := range algorithms {
		algorithms[i] = normalizeAlgorithm(algorithms[i])
	}

	cfg := action.HashConfig{
		FilePath:   filePath,
		Algorithms: algorithms,
	}

	log.Info().
		Str("file", filePath).
		Strs("algorithms", algorithms).
		Msg("Starting hash calculation")

	results, err := action.HashFile(ctx, cfg)
	if err != nil {
		if errors.Is(err, context.Canceled) {
			log.Warn().Msg("Hash calculation canceled")
			return &ExitError{Code: 130, Err: context.Canceled}
		}

		return &ExitError{Code: 2, Err: fmt.Errorf("failed to calculate hash: %w", err)}
	}

	for _, result := range results {
		log.Info().
			Str("algorithm", result.Algorithm.String()).
			Str("hash", result.Hash).
			Msg("Calculated")
	}

	log.Info().
		Str("file", filePath).
		Int("algorithms", len(results)).
		Msg("Hash calculation completed")

	return nil
}

func newHashCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "hash <file>",
		Short: "Calculate hash of a single file",
		Long: strings.Trim(dedent.Dedent(`
			Calculate hash of a single file using algorithms specified in configuration.
			Algorithms can be configured via hash.algorithms setting.
			Use --algorithms to override the configuration for a single invocation
			(repeatable, or comma-separated, e.g. --algorithms .md5 --algorithms .sha256 or --algorithms .md5,.sha256).

			Supported algorithms: .sfv (CRC32), .md4, .md5, .sha1, .sha256, .sha384, .sha512, .sha3-256, .sha3-384, .sha3-512, .blake3, .xxh3, .xxh128.`,
		), "\n"),
		Args: cobra.ExactArgs(1),
		RunE: runHash,
	}

	cmd.Flags().StringSlice("algorithms", nil, "comma-separated or repeatable list of algorithm extensions (overrides hash.algorithms)")

	return cmd
}
