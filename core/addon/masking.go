package addon

import (
	"encoding/json"

	"github.com/goto/entropy/pkg/errors"
)

// MaskPlaceholder replaces sensitive addon-config values on read.
const MaskPlaceholder = "****"

var maskedValue = json.RawMessage(`"` + MaskPlaceholder + `"`)

func maskConfigs(configs json.RawMessage, keys []string) (json.RawMessage, error) {
	if len(keys) == 0 {
		return configs, nil
	}

	m, err := toObject(configs)
	if err != nil {
		return nil, err
	}
	for _, key := range keys {
		if _, found := m[key]; found {
			m[key] = maskedValue
		}
	}
	return json.Marshal(m)
}

// restoreConfigs replaces sensitive keys sent as the mask placeholder with the
// stored value, or drops them when nothing is stored.
func restoreConfigs(incoming, stored json.RawMessage, keys []string) (json.RawMessage, error) {
	in, err := toObject(incoming)
	if err != nil {
		return nil, err
	}

	var prev map[string]json.RawMessage
	if len(stored) > 0 {
		if prev, err = toObject(stored); err != nil {
			return nil, err
		}
	}

	for _, key := range keys {
		var s string
		if v, found := in[key]; !found || json.Unmarshal(v, &s) != nil || s != MaskPlaceholder {
			continue
		}

		if v, found := prev[key]; found {
			in[key] = v
		} else {
			delete(in, key)
		}
	}
	return json.Marshal(in)
}

func toObject(raw json.RawMessage) (map[string]json.RawMessage, error) {
	var m map[string]json.RawMessage
	if err := json.Unmarshal(raw, &m); err != nil || m == nil {
		return nil, errors.ErrInvalid.WithMsgf("configs must be a JSON object")
	}
	return m, nil
}
