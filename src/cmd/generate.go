package cmd

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

	"github.com/ostapkonst/HashVerifier/internal/action"
	"github.com/ostapkonst/HashVerifier/internal/checksum"
	"github.com/ostapkonst/HashVerifier/internal/output"
	"github.com/ostapkonst/HashVerifier/internal/settings"
	"github.com/ostapkonst/HashVerifier/utils/gracer"
)

func runGenerate(cmd *cobra.Command, args []string) error {
	ctx, cancel := context.WithCancel(cmd.Context())
	defer cancel()

	excludePaths, err := cmd.Flags().GetStringArray("exclude")
	if err != nil {
		err = fmt.Errorf("internal error reading --exclude flag: %w", err)
		return &ExitError{Code: 2, Err: err}
	}

	done := make(chan error, 1)

	gracer.AddCallback(func() error {
		cancel()
		return <-done
	})

	go func() {
		done <- execGenerate(ctx, cmd, args, excludePaths)

		gracer.GracefulShutdown()
	}()

	return gracer.Wait()
}

func execGenerate(ctx context.Context, cmd *cobra.Command, args []string, excludePaths []string) error {
	inputDir := filepath.Clean(args[0])
	outputFile := filepath.Clean(args[1])

	force, _ := cmd.Flags().GetBool("force")
	if err := output.ShouldOverwrite(outputFile, force); err != nil {
		if errors.Is(err, output.ErrRefuseOverwrite) {
			return &ExitError{
				Code: 1,
				Err:  fmt.Errorf("refusing to overwrite existing file: %s (use --force)", outputFile),
			}
		}

		return &ExitError{Code: 1, Err: fmt.Errorf("invalid output file: %w", err)}
	}

	cfgSettings := loadAndLog(cmd)

	algorithm, err := resolveAlgorithm(cmd, outputFile, cfgSettings)
	if err != nil {
		return &ExitError{Code: 1, Err: fmt.Errorf("failed to resolve algorithm: %w", err)}
	}

	flatPaths := flagBoolOrDefault(cmd, "flat-paths", cfgSettings.Generate.FlatPaths)

	dirPrefix := ""
	if !flatPaths {
		dirPrefix, err = checksum.GetPrefixForFilesInChecksum(inputDir, outputFile)
		if err != nil {
			return &ExitError{Code: 1, Err: fmt.Errorf("failed to get prefix: %w", err)}
		}
	}

	cfg := action.GenerateConfig{
		InputDir:            inputDir,
		OutputFile:          outputFile,
		Algorithm:           algorithm,
		DirPrefix:           dirPrefix,
		FollowSymbolicLinks: flagBoolOrDefault(cmd, "follow-symbolic-links", cfgSettings.Generate.FollowSymbolicLinks),
		SortPaths:           flagBoolOrDefault(cmd, "sort-paths", cfgSettings.Generate.SortPaths),
		FlatPaths:           flatPaths,
		ExcludeMatcher:      checksum.NewExcludeMatcher(excludePaths),
		OnFileHashed: func(res checksum.GenerateResult) {
			commonFields := func(event *zerolog.Event, err error) *zerolog.Event {
				logger := event.
					Str("file", res.RelPath).
					Str("status", res.Status.String()).
					Str("hash", res.Hash).
					Str("size", bytesize.New(float64(res.ReadBytes)).String())

				if err != nil {
					logger = logger.Err(err)
				}

				return logger
			}

			switch res.Status {
			case checksum.GenSuccess:
				commonFields(log.Info(), nil).Msg("Hashed")
			case checksum.GenSkipped:
				commonFields(log.Warn(), res.Err).Msg("Skipped file")
			default:
				commonFields(log.Error(), res.Err).Msg("Failed to hash file")
			}
		},
	}

	log.Info().
		Str("input_dir", inputDir).
		Str("output_file", outputFile).
		Str("algorithm", algorithm.String()).
		Str("dir_prefix", cfg.DirPrefix).
		Bool("follow_symbolic_links", cfg.FollowSymbolicLinks).
		Bool("sort_paths", cfg.SortPaths).
		Bool("flat_paths", cfg.FlatPaths).
		Strs("exclude", excludePaths).
		Msg("Starting generation")

	result, err := action.GenerateChecksums(ctx, cfg)
	if err != nil {
		if errors.Is(err, context.Canceled) {
			log.Warn().Msg("Checksum generation canceled")
			return &ExitError{Code: 130, Err: context.Canceled}
		}

		return &ExitError{Code: 2, Err: fmt.Errorf("failed to generate checksums: %w", err)}
	}

	stats := result.Stats
	log.Info().
		Int("processed", stats.Processed).
		Int("skipped", stats.Skipped).
		Int("pending", stats.Pending()).
		Int("with_errors", stats.WithErrors).
		Int("total_files", stats.TotalFiles).
		Msg("Generation stats")

	log.Info().
		Str("output_file", outputFile).
		Msg("Generation completed")

	if stats.WithErrors > 0 {
		return &ExitError{
			Code: 1,
			Err:  fmt.Errorf("%d of %d files failed to hash", stats.WithErrors, stats.TotalFiles),
		}
	}

	return nil
}

func resolveAlgorithm(cmd *cobra.Command, outputFile string, cfg *settings.Settings) (checksum.Algorithm, error) {
	if cmd.Flags().Changed("algorithm") {
		raw, err := cmd.Flags().GetString("algorithm")
		if err != nil {
			return checksum.Unknown, fmt.Errorf("internal error reading --algorithm flag: %w", err)
		}

		if raw != "" {
			return checksum.AlgorithmFromExtension(raw)
		}
	}

	if algo, err := checksum.AlgorithmFromExtension(outputFile); err == nil {
		return algo, nil
	}

	return checksum.AlgorithmFromExtension(cfg.Generate.Algorithm)
}

func newGenerateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "generate <input_dir> <checksum_file>",
		Short: "Generate checksum file recursively from directory",
		Long: strings.Trim(dedent.Dedent(`
			Generate checksum file recursively from directory.
			Algorithm is determined in this order: --algorithm flag, output file extension, generate.algorithm config setting.
			Settings generate.follow_symbolic_links, generate.sort_paths and generate.flat_paths are loaded from configuration file.
			CLI flags override their respective config settings when passed explicitly.

			Use --exclude to skip specific files or directories relative to <input_dir>.
			Directories should end with a path separator (e.g. --exclude 'build/').
			Repeat the flag to exclude multiple paths.

			Use --flat-paths to strip the root directory name from paths in the checksum file.
			The checksum file should be saved inside the source directory when this flag is used.

			Supported algorithms: .sfv (CRC32), .md4, .md5, .sha1, .sha256, .sha384, .sha512, .sha3-256, .sha3-384, .sha3-512, .blake3, .xxh3, .xxh128.`,
		), "\n"),
		Args: cobra.ExactArgs(2),
		RunE: runGenerate,
	}

	cmd.Flags().StringArray("exclude", nil, "exclude relative path from generation (repeatable; append '/' for directories)")
	cmd.Flags().String("algorithm", "", "hash algorithm with leading dot (e.g., .sha256, .md5, .sfv); overrides output extension detection and generate.algorithm config setting")
	cmd.Flags().Bool("force", false, "overwrite existing output file without prompting")

	addOptBoolFlag(cmd, "follow-symbolic-links", false, "follow symbolic links when scanning directories (default from generate.follow_symbolic_links)")
	addOptBoolFlag(cmd, "sort-paths", false, "sort paths before hashing (default from generate.sort_paths)")
	addOptBoolFlag(cmd, "flat-paths", false, "strip root directory from paths; save checksum file inside source directory (default from generate.flat_paths)")

	return cmd
}
