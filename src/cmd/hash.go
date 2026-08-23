package cmd

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

	"github.com/ostapkonst/HashVerifier/internal/action"
	"github.com/ostapkonst/HashVerifier/internal/checksum"
	"github.com/ostapkonst/HashVerifier/internal/header"
	"github.com/ostapkonst/HashVerifier/internal/output"
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

	rawAlgorithms := flagStringSliceOrDefault(cmd, "algorithms", cfgSettings.Hash.Algorithms)

	seen := make(map[checksum.Algorithm]struct{}, len(rawAlgorithms))

	algos := make([]checksum.Algorithm, 0, len(rawAlgorithms))

	algoStrings := make([]string, 0, len(rawAlgorithms))
	for _, raw := range rawAlgorithms {
		algo, err := checksum.AlgorithmFromExtension(raw)
		if err != nil {
			return &ExitError{Code: 1, Err: fmt.Errorf("unsupported algorithm %q: %w", raw, err)}
		}

		if _, ok := seen[algo]; ok {
			continue
		}

		seen[algo] = struct{}{}

		algos = append(algos, algo)
		algoStrings = append(algoStrings, algo.String())
	}

	cfg := action.HashConfig{
		FilePath:   filePath,
		Algorithms: algos,
	}

	log.Info().
		Str("file", filePath).
		Strs("algorithms", algoStrings).
		Msg("Starting hashing")

	result, err := action.HashFile(ctx, cfg)
	if err != nil {
		if errors.Is(err, context.Canceled) {
			log.Warn().Msg("Hash calculation canceled")
			return &ExitError{Code: 130, Err: context.Canceled}
		}

		return &ExitError{Code: 1, Err: fmt.Errorf("failed to calculate hash: %w", err)}
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
				return &ExitError{Code: 1, Err: err}
			}
		}
	}

	return nil
}

func writeChecksumLine(result action.HashResult, sourcePath, outputPath string, force bool) error {
	algo, err := checksum.AlgorithmFromExtension(outputPath)
	if err != nil {
		return fmt.Errorf("cannot determine algorithm from extension of %s: %w", outputPath, err)
	}

	hashStr, ok := result.Hashes[algo]
	if !ok {
		return fmt.Errorf("algorithm %s not calculated; add it to --algorithms", algo.String())
	}

	if err := output.ShouldOverwrite(outputPath, force); err != nil {
		if errors.Is(err, output.ErrRefuseOverwrite) {
			return fmt.Errorf("refusing to overwrite existing file: %s (use --force)", outputPath)
		}

		return fmt.Errorf("invalid output file: %w", err)
	}

	line := checksum.FormatLine(filepath.Base(sourcePath), hashStr, algo)
	if err := os.WriteFile(outputPath, []byte(header.FormatExportedFile(line)), 0o644); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	log.Info().
		Str("file", outputPath).
		Str("algorithm", algo.String()).
		Str("hash", hashStr).
		Msg("Hash exported to checksum file")

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

	cmd.Flags().StringSlice("algorithms", nil, "comma-separated or repeatable list of algorithm extensions with leading dots (overrides hash.algorithms)")
	cmd.Flags().StringArray("export", nil, "write checksum line to file (repeatable; algorithm determined by extension; requires matching --algorithms entry)")
	cmd.Flags().Bool("force", false, "overwrite existing output file without prompting")

	return cmd
}
