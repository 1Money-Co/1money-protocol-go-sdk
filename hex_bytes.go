package onemoney

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
)

// HexBytes marshals to/from 0x-prefixed hex strings in JSON while storing raw bytes.
type HexBytes []byte

// MarshalJSON encodes the bytes as a 0x-prefixed hex string.
func (hb HexBytes) MarshalJSON() ([]byte, error) {
	if len(hb) == 0 {
		return json.Marshal("0x")
	}
	return json.Marshal("0x" + hex.EncodeToString(hb))
}

// UnmarshalJSON decodes a 0x-prefixed hex string into bytes.
func (hb *HexBytes) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		*hb = nil
		return nil
	}

	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	s = strings.TrimPrefix(s, "0x")
	if s == "" {
		*hb = HexBytes{}
		return nil
	}

	b, err := hex.DecodeString(s)
	if err != nil {
		return fmt.Errorf("decode hex string: %w", err)
	}
	*hb = HexBytes(b)
	return nil
}
