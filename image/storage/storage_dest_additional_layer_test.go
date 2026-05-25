//go:build !containers_image_storage_stub

package storage

import (
	"testing"

	digest "github.com/opencontainers/go-digest"
	"github.com/stretchr/testify/assert"
	cstorage "go.podman.io/storage"
)

func TestAdditionalLayerCandidatesIncludesFallbackDigests(t *testing.T) {
	tocDigest := digest.FromString("toc")
	compressedDigest := digest.FromString("compressed")
	uncompressedDigest := digest.FromString("diff")

	candidates := additionalLayerCandidates(tocDigest, true, compressedDigest, uncompressedDigest)

	assert.Equal(t, []string{
		tocDigest.String(),
		tocDigest.Encoded(),
		compressedDigest.String(),
		compressedDigest.Encoded(),
		uncompressedDigest.String(),
		uncompressedDigest.Encoded(),
	}, candidateKeys(candidates))
}

func TestAdditionalLayerCandidatesSkipsTOCWhenDisabled(t *testing.T) {
	compressedDigest := digest.FromString("compressed")
	uncompressedDigest := digest.FromString("diff")

	candidates := additionalLayerCandidates(digest.FromString("toc"), false, compressedDigest, uncompressedDigest)

	assert.Equal(t, []string{
		compressedDigest.String(),
		compressedDigest.Encoded(),
		uncompressedDigest.String(),
		uncompressedDigest.Encoded(),
	}, candidateKeys(candidates))
}

func TestAdditionalLayerCandidatesDeduplicatesKeys(t *testing.T) {
	sharedDigest := digest.FromString("shared")

	candidates := additionalLayerCandidates(sharedDigest, true, sharedDigest, sharedDigest)

	assert.Equal(t, []string{
		sharedDigest.String(),
		sharedDigest.Encoded(),
	}, candidateKeys(candidates))
}

func candidateKeys(candidates []cstorage.AdditionalLayerCandidate) []string {
	keys := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		keys = append(keys, candidate.Key)
	}
	return keys
}
