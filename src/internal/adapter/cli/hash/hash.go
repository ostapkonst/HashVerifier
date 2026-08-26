// Package hash implements the `hashverifier hash` subcommand.
package hash

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/lithammer/dedent"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	"github.com/ostapkonst/HashVerifier/internal/adapter/cli/base"
	"github.com/ostapkonst/HashVerifier/internal/appmeta"
	"github.com/ostapkonst/HashVerifier/internal/domain/algorithm"
	"github.com/ostapkonst/HashVerifier/internal/domain/walk"
	"github.com/ostapkonst/HashVerifier/internal/platform/fs"
	servicehash "github.com/ostapkonst/HashVerifier/internal/service/hash"
)

func runHash(cmd *cobra.Command, args []string) error {
	return base.RunWithShutdown(cmd, func(ctx context.Context) error {
		return execHash(ctx, cmd, args)
	})
}

func execHash(ctx context.Context, cmd *cobra.Command, args []string) error {
	filePath := filepath.Clean(args[0])

	cfgSettings := base.LoadAndLog(cmd)

	rawAlgorithms := base.FlagStringSliceOrDefault(cmd, "algorithms", cfgSettings.Hash.Algorithms)

	algos, algoStrings, err := base.ParseAlgorithms(rawAlgorithms)
	if err != nil {
		return &base.ExitError{Code: 1, Err: fmt.Errorf("parsing --algorithms: %w", err)}
	}

	cfg := servicehash.HashConfig{
		FilePath:   filePath,
		Algorithms: algos,
	}

	log.Info().
		Str("file", filePath).
		Strs("algorithms", algoStrings).
		Msg("Starting hashing")

	result, err := servicehash.HashFile(ctx, cfg)
	if err != nil {
		if errors.Is(err, context.Canceled) {
			log.Warn().Msg("Hash calculation canceled")
			return &base.ExitError{Code: 130, Err: context.Canceled}
		}

		return &base.ExitError{Code: 1, Err: fmt.Errorf("failed to calculate hash: %w", err)}
	}

	for algo, hash := range result.Hashes {
		log.Info().
			Str("algorithm", algo.String()).
			Str("hash", hash).
			Msg("Calculated")
	}

	log.Info().
		Str("file", filePath).
		Int("algorithms", len(result.Hashes)).
		Msg("Hashing completed")

	exports, _ := cmd.Flags().GetStringArray("export")
	if len(exports) > 0 {
		seen := make(map[string]struct{}, len(exports))
		force, _ := cmd.Flags().GetBool("force")

		for _, path := range exports {
			if _, ok := seen[path]; ok {
				continue
			}

			seen[path] = struct{}{}

			if err := writeChecksumLine(result, filePath, path, force); err != nil {
				return &base.ExitError{Code: 1, Err: err}
			}
		}
	}

	return nil
}

func writeChecksumLine(result servicehash.HashResult, sourcePath, outputPath string, force bool) error {
	algo, err := algorithm.AlgorithmFromExtension(outputPath)
	if err != nil {
		return fmt.Errorf("cannot determine algorithm from extension of %s: %w", outputPath, err)
	}

	hashStr, ok := result.Hashes[algo]
	if !ok {
		return fmt.Errorf("algorithm %s not calculated; add it to --algorithms", algo.String())
	}

	if err := fs.ShouldOverwrite(outputPath, force); err != nil {
		if errors.Is(err, fs.ErrRefuseOverwrite) {
			return fmt.Errorf("refusing to overwrite existing file: %s: %w", outputPath, fs.ErrRefuseOverwrite)
		}

		return fmt.Errorf("invalid output file: %w", err)
	}

	line := walk.FormatLine(filepath.Base(sourcePath), hashStr, algo)
	if err := os.WriteFile(outputPath, []byte(appmeta.FormatExportedFile(line)), 0o644); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	log.Info().
		Str("file", outputPath).
		Str("algorithm", algo.String()).
		Str("hash", hashStr).
		Msg("Hash exported to checksum file")

	return nil
}

// NewCmd assembles the hash subcommand and its flags for the root command.
func NewCmd() *cobra.Command {
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

	cmd.Flags().StringSlice("algorithms", nil, "comma-separated or repeatable list of algorithm extensions with leading dots (overrides hash.algorithms)")
	cmd.Flags().StringArray("export", nil, "write checksum line to file (repeatable; algorithm determined by extension; requires matching --algorithms entry)")
	cmd.Flags().Bool("force", false, "overwrite existing output file without prompting")

	return cmd
}
