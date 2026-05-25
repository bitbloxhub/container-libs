//go:build linux

package storage

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"testing"

	digest "github.com/opencontainers/go-digest"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupAdditionalLayerStore creates a temporary additional layer store directory
// with the expected structure for the given lookup key and image reference.
// The info file contains the provided content.
func setupAdditionalLayerStore(t *testing.T, key string, imageRef string, infoContent string) string {
	t.Helper()
	alsRoot := t.TempDir()

	refDir := base64.StdEncoding.EncodeToString([]byte(imageRef))
	layerDir := filepath.Join(alsRoot, refDir, key)
	require.NoError(t, os.MkdirAll(layerDir, 0o755))

	require.NoError(t, os.MkdirAll(filepath.Join(layerDir, "diff"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(layerDir, "info"), []byte(infoContent), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(layerDir, "blob"), []byte{}, 0o644))

	return alsRoot
}

func TestLookupAdditionalLayerSuccess(t *testing.T) {
	prevLevel := logrus.GetLevel()
	logrus.SetLevel(logrus.ErrorLevel)
	t.Cleanup(func() { logrus.SetLevel(prevLevel) })

	tocDigest := digest.FromString("test-layer")
	imageRef := "fedora"

	info := Layer{
		ID:             "test-id",
		CompressedSize: 42,
		TOCDigest:      tocDigest,
	}
	infoJSON, err := json.Marshal(info)
	require.NoError(t, err)

	alsPath := setupAdditionalLayerStore(t, tocDigest.String(), imageRef, string(infoJSON))
	store := newTestStore(t, StoreOptions{
		GraphDriverName:    "overlay",
		GraphDriverOptions: []string{"additionallayerstore=" + alsPath + ":ref"},
	})
	t.Cleanup(func() { _, _ = store.Shutdown(true) })

	al, err := store.LookupAdditionalLayer(tocDigest, imageRef)
	require.NoError(t, err)
	defer al.Release()

	assert.Equal(t, tocDigest, al.TOCDigest())
	assert.Equal(t, int64(42), al.CompressedSize())
}

func TestLookupAdditionalLayerDecodeError(t *testing.T) {
	prevLevel := logrus.GetLevel()
	logrus.SetLevel(logrus.ErrorLevel)
	t.Cleanup(func() { logrus.SetLevel(prevLevel) })

	tocDigest := digest.FromString("test-layer")
	imageRef := "fedora"

	alsPath := setupAdditionalLayerStore(t, tocDigest.String(), imageRef, "not valid json")
	store := newTestStore(t, StoreOptions{
		GraphDriverName:    "overlay",
		GraphDriverOptions: []string{"additionallayerstore=" + alsPath + ":ref"},
	})
	t.Cleanup(func() { _, _ = store.Shutdown(true) })

	_, err := store.LookupAdditionalLayer(tocDigest, imageRef)
	assert.Error(t, err, "should fail on invalid JSON in info file")
}

func TestLookupAdditionalLayerByCandidatesFallsBackToDigestKey(t *testing.T) {
	prevLevel := logrus.GetLevel()
	logrus.SetLevel(logrus.ErrorLevel)
	t.Cleanup(func() { logrus.SetLevel(prevLevel) })

	tocDigest := digest.FromString("missing-toc")
	compressedDigest := digest.FromString("compressed-layer")
	imageRef := "fedora"

	info := Layer{
		ID:               "test-id",
		CompressedSize:   42,
		TOCDigest:        tocDigest,
		CompressedDigest: compressedDigest,
	}
	infoJSON, err := json.Marshal(info)
	require.NoError(t, err)

	alsPath := setupAdditionalLayerStore(t, compressedDigest.String(), imageRef, string(infoJSON))
	store := newTestStore(t, StoreOptions{
		GraphDriverName:    "overlay",
		GraphDriverOptions: []string{"additionallayerstore=" + alsPath + ":ref"},
	})
	t.Cleanup(func() { _, _ = store.Shutdown(true) })

	al, err := store.LookupAdditionalLayerByCandidates([]AdditionalLayerCandidate{
		{Key: tocDigest.String(), Kind: "toc"},
		{Key: compressedDigest.String(), Kind: "compressed-digest"},
	}, imageRef)
	require.NoError(t, err)
	defer al.Release()

	assert.Equal(t, tocDigest, al.TOCDigest())
	assert.Equal(t, int64(42), al.CompressedSize())
}

func TestLookupAdditionalLayerByCandidatesPrefersFirstMatch(t *testing.T) {
	prevLevel := logrus.GetLevel()
	logrus.SetLevel(logrus.ErrorLevel)
	t.Cleanup(func() { logrus.SetLevel(prevLevel) })

	firstDigest := digest.FromString("first-layer")
	secondDigest := digest.FromString("second-layer")
	imageRef := "fedora"

	firstInfoJSON, err := json.Marshal(Layer{
		ID:             "first-id",
		CompressedSize: 1,
		TOCDigest:      firstDigest,
	})
	require.NoError(t, err)

	secondInfoJSON, err := json.Marshal(Layer{
		ID:             "second-id",
		CompressedSize: 2,
		TOCDigest:      secondDigest,
	})
	require.NoError(t, err)

	alsPath := setupAdditionalLayerStore(t, firstDigest.String(), imageRef, string(firstInfoJSON))
	refDir := base64.StdEncoding.EncodeToString([]byte(imageRef))
	secondLayerDir := filepath.Join(alsPath, refDir, secondDigest.String())
	require.NoError(t, os.MkdirAll(filepath.Join(secondLayerDir, "diff"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(secondLayerDir, "info"), []byte(secondInfoJSON), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(secondLayerDir, "blob"), []byte{}, 0o644))

	store := newTestStore(t, StoreOptions{
		GraphDriverName:    "overlay",
		GraphDriverOptions: []string{"additionallayerstore=" + alsPath + ":ref"},
	})
	t.Cleanup(func() { _, _ = store.Shutdown(true) })

	al, err := store.LookupAdditionalLayerByCandidates([]AdditionalLayerCandidate{
		{Key: firstDigest.String(), Kind: "first"},
		{Key: secondDigest.String(), Kind: "second"},
	}, imageRef)
	require.NoError(t, err)
	defer al.Release()

	assert.Equal(t, firstDigest, al.TOCDigest())
	assert.Equal(t, int64(1), al.CompressedSize())
}
