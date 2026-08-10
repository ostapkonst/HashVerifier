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
	"github.com/ostapkonst/HashVerifier/internal/settings"
	"github.com/ostapkonst/HashVerifier/utils/gracer"
)

func runGenerate(cmd *cobra.Command, args []string) error {
	ctx, cancel := context.WithCancel(cmd.Context())
	defer cancel()

	excludePaths, err := cmd.Flags().GetStringArray("exclude")
	if err != nil {
		return fmt.Errorf("internal error reading --exclude flag: %w", err)
	}

	flatPaths, err := cmd.Flags().GetBool("flat-paths")
	if err != nil {
		return fmt.Errorf("internal error reading --flat-paths flag: %w", err)
	}

	done := make(chan error, 1)

	gracer.AddCallback(func() error {
		cancel()
		return <-done
	})

	go func() {
		done <- execGenerate(ctx, args, excludePaths, flatPaths)

		gracer.GracefulShutdown()
	}()

	return gracer.Wait()
}

func execGenerate(ctx context.Context, args []string, excludePaths []string, flatPaths bool) error {
	inputDir := filepath.Clean(args[0])
	outputFile := filepath.Clean(args[1])

	cfgSettings, err := settings.Load()
	if err != nil {
		log.Warn().Err(err).Msg("Failed to load settings, using defaults")

		cfgSettings = settings.DefaultSettings()
	}

	cfg := action.GenerateConfig{
		InputDir:            inputDir,
		OutputFile:          outputFile,
		FollowSymbolicLinks: cfgSettings.Generate.FollowSymbolicLinks,
		SortPaths:           cfgSettings.Generate.SortPaths,
		FlatPaths:           flatPaths,
		ExcludeRelPaths:     excludePaths,
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
		Msg("Starting checksum generation")

	result, err := action.GenerateChecksums(ctx, cfg)
	if err != nil {
		if errors.Is(err, context.Canceled) {
			log.Warn().Msg("Checksum generation canceled")
			return nil
		}

		return fmt.Errorf("failed to generate checksums: %w", err)
	}

	stats := result.Stats
	log.Info().
		Int("processed", stats.Processed).
		Int("skipped", stats.Skipped).
		Int("pending", stats.Pending()).
		Int("with_errors", stats.WithErrors).
		Int("total_files", stats.TotalFiles).
		Msg("Checksum generation stats")

	log.Info().
		Str("file", outputFile).
		Msg("Checksum generation completed")

	return nil
}

var generateCmd = &cobra.Command{
	Use:   "generate <input_dir> <checksum_file>",
	Short: "Generate checksum file recursively from directory",
	Long: strings.Trim(dedent.Dedent(`
		Generate checksum file recursively from directory.
		Algorithm is determined automatically from file extension.
		Settings generate.follow_symbolic_links and generate.sort_paths are loaded from configuration file.

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

func init() {
	rootCmd.AddCommand(generateCmd)

	generateCmd.Flags().StringArrayP("exclude", "e", nil, "exclude relative path from generation (repeatable; append '/' for directories)")
	generateCmd.Flags().Bool("flat-paths", false, "strip root directory from paths; save checksum file inside source directory")
}
