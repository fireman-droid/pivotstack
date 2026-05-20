package db

import (
	"encoding/json"
	"fmt"
)

func jsonUnmarshalObject(raw []byte, dst any) error {
	if len(raw) == 0 {
		raw = []byte("{}")
	}
	if string(raw) == "null" {
		raw = []byte("{}")
	}
	if err := json.Unmarshal(raw, dst); err != nil {
		return fmt.Errorf("unmarshal jsonb object: %w", err)
	}
	return nil
}
