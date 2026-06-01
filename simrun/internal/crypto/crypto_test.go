package crypto

import (
	"encoding/base64"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEncryptor_Roundtrip(t *testing.T) {
	keyPath := filepath.Join(t.TempDir(), "test.key")
	enc, err := LoadOrGenerateKey(keyPath)
	require.NoError(t, err)

	cases := []string{"", "short", "the quick brown fox jumps over the lazy dog", string(make([]byte, 4096))}
	for _, plain := range cases {
		ciphertext, err := enc.Encrypt(plain)
		require.NoError(t, err)
		assert.NotEqual(t, plain, ciphertext, "ciphertext should differ from plaintext")

		got, err := enc.Decrypt(ciphertext)
		require.NoError(t, err)
		assert.Equal(t, plain, got)
	}
}

func TestEncryptor_DecryptRejectsTamperedCiphertext(t *testing.T) {
	enc, err := LoadOrGenerateKey(filepath.Join(t.TempDir(), "test.key"))
	require.NoError(t, err)

	ct, err := enc.Encrypt("secret")
	require.NoError(t, err)

	raw, err := base64.StdEncoding.DecodeString(ct)
	require.NoError(t, err)
	raw[len(raw)/2] ^= 0xFF

	_, err = enc.Decrypt(base64.StdEncoding.EncodeToString(raw))
	assert.Error(t, err)
}

func TestEncryptor_DecryptRejectsInvalidBase64(t *testing.T) {
	enc, err := LoadOrGenerateKey(filepath.Join(t.TempDir(), "test.key"))
	require.NoError(t, err)

	_, err = enc.Decrypt("!!!not-base64!!!")
	assert.Error(t, err)
}

func TestLoadOrGenerateKey_PersistsAcrossLoads(t *testing.T) {
	keyPath := filepath.Join(t.TempDir(), "test.key")
	enc1, err := LoadOrGenerateKey(keyPath)
	require.NoError(t, err)

	ct, err := enc1.Encrypt("hello")
	require.NoError(t, err)

	enc2, err := LoadOrGenerateKey(keyPath)
	require.NoError(t, err)

	got, err := enc2.Decrypt(ct)
	require.NoError(t, err)
	assert.Equal(t, "hello", got)
}

func TestNewEncryptor_RejectsBadKeys(t *testing.T) {
	cases := []struct {
		name string
		key  string
	}{
		{"invalid base64", "!!!"},
		{"wrong length", base64.StdEncoding.EncodeToString([]byte("too-short"))},
		{"empty", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := NewEncryptor(tc.key)
			assert.Error(t, err)
		})
	}
}
