// issue-02.3 — key derivation y keyring multi-versión desde env.
package crypto

import (
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// DeriveKey normaliza cualquier input a una master key de 32 bytes.
// Input de exactamente 32 bytes pasa sin cambios; otros largos se derivan
// con SHA-256 (determinístico).
func DeriveKey(input []byte) []byte {
	if len(input) == MasterKeySize {
		return input
	}
	sum := sha256.Sum256(input)
	return sum[:]
}

// LoadKeyring construye un Cipher desde el formato multi-versión
// "1:<base64>,2:<base64>" (env DOMAIN_MASTER_KEYS). La versión más alta
// queda como current para Encrypt; las anteriores solo para Decrypt.
// Necesario post-rotation: los blobs viejos llevan su version byte inline.
func LoadKeyring(spec string) (*Cipher, error) {
	type entry struct {
		version byte
		key     []byte
	}
	var entries []entry
	for _, part := range strings.Split(spec, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		vRaw, b64, ok := strings.Cut(part, ":")
		if !ok {
			return nil, fmt.Errorf("keyring entry %q: expected <version>:<base64>", part)
		}
		v, err := strconv.Atoi(strings.TrimSpace(vRaw))
		if err != nil || v < 1 || v > 255 {
			return nil, fmt.Errorf("keyring entry %q: version must be 1..255", part)
		}
		raw, err := base64.StdEncoding.DecodeString(strings.TrimSpace(b64))
		if err != nil {
			return nil, fmt.Errorf("keyring entry %q: %w", part, err)
		}
		if len(raw) != MasterKeySize {
			return nil, fmt.Errorf("keyring entry %q: %w", part, ErrInvalidKeySize)
		}
		entries = append(entries, entry{version: byte(v), key: raw})
	}
	if len(entries) == 0 {
		return nil, fmt.Errorf("keyring spec empty")
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].version < entries[j].version })

	c, err := newCipherWithVersion(entries[0].version, entries[0].key)
	if err != nil {
		return nil, err
	}
	for _, e := range entries[1:] {
		if err := c.AddKey(e.version, e.key); err != nil {
			return nil, err
		}
	}
	return c, nil
}

// newCipherWithVersion crea el cipher base con una versión arbitraria
// (NewCipher asume version 1; el keyring puede arrancar más arriba).
func newCipherWithVersion(version byte, masterKey []byte) (*Cipher, error) {
	c, err := NewCipher(masterKey)
	if err != nil {
		return nil, err
	}
	if version != 1 {
		c.keyring[version] = c.keyring[1]
		delete(c.keyring, 1)
		c.current = version
	}
	return c, nil
}
