package action

import (
	"context"

	"github.com/ostapkonst/HashVerifier/internal/action/generator"
	"github.com/ostapkonst/HashVerifier/internal/action/hasher"
	"github.com/ostapkonst/HashVerifier/internal/action/verifier"
)

type (
	GenerateConfig          = generator.GenerateConfig
	GenerateResultStats     = generator.GenerateResultStats
	GenerateStreamingConfig = generator.GenerateStreamingConfig
	GenerateStreamingResult = generator.GenerateStreamingResult
)

type (
	VerifyConfig          = verifier.VerifyConfig
	VerifyResultStats     = verifier.VerifyResultStats
	VerifyStreamingConfig = verifier.VerifyStreamingConfig
	VerifyStreamingResult = verifier.VerifyStreamingResult
)

type (
	HashConfig          = hasher.HashConfig
	HashResult          = hasher.HashResult
	HashStreamingConfig = hasher.HashConfig // тот же тип, что и HashConfig: streaming-вариант не добавляет новых полей
	HashStreamingResult = hasher.HashStreamingResult
)

func GenerateChecksums(ctx context.Context, cfg GenerateConfig) (GenerateResultStats, error) {
	return generator.GenerateChecksums(ctx, cfg)
}

func GenerateChecksumsStreamingToFile(ctx context.Context, cfg GenerateStreamingConfig) (<-chan GenerateStreamingResult, error) {
	return generator.GenerateChecksumsStreamingToFile(ctx, cfg)
}

func VerifyChecksums(ctx context.Context, cfg VerifyConfig) (VerifyResultStats, error) {
	return verifier.VerifyChecksums(ctx, cfg)
}

func VerifyChecksumsStreaming(ctx context.Context, cfg VerifyStreamingConfig) (<-chan VerifyStreamingResult, error) {
	return verifier.VerifyChecksumsStreaming(ctx, cfg)
}

func HashFile(ctx context.Context, cfg HashConfig) ([]HashResult, error) {
	return hasher.HashFile(ctx, cfg)
}

func HashFileStreaming(ctx context.Context, cfg HashStreamingConfig) (<-chan HashStreamingResult, error) {
	return hasher.HashFileStreaming(ctx, cfg)
}
