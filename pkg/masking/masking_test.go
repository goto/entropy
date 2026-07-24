package masking

import (
	"encoding/json"
	"testing"
)

var testKey = []byte("test-hmac-key")

// unmarshal is a test helper.
func unmarshal(t *testing.T, raw json.RawMessage) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	return m
}

func TestMask_DotPath(t *testing.T) {
	m := New(testKey)
	in := json.RawMessage(`{"env_variables":{"SOURCE_KAFKA_PASSWORD":"hunter2","REPLICAS":"3"}}`)

	out, err := m.Mask(in, []string{"env_variables.SOURCE_KAFKA_PASSWORD"})
	if err != nil {
		t.Fatal(err)
	}

	got := unmarshal(t, out)
	env := got["env_variables"].(map[string]any)
	if s := env["SOURCE_KAFKA_PASSWORD"].(string); s[:5] != maskPrefix {
		t.Errorf("password not masked: %q", s)
	}
	if env["REPLICAS"] != "3" {
		t.Errorf("non-sensitive field changed: %v", env["REPLICAS"])
	}
}

func TestMask_NestedPath(t *testing.T) {
	m := New(testKey)
	in := json.RawMessage(`{"telegraf":{"config":{"output":{"password":"secret"}}}}`)

	out, err := m.Mask(in, []string{"telegraf.config.output.password"})
	if err != nil {
		t.Fatal(err)
	}
	got := unmarshal(t, out)
	pwd := got["telegraf"].(map[string]any)["config"].(map[string]any)["output"].(map[string]any)["password"].(string)
	if pwd[:5] != maskPrefix {
		t.Errorf("nested password not masked: %q", pwd)
	}
}

func TestMask_Wildcard(t *testing.T) {
	m := New(testKey)
	in := json.RawMessage(`{"env_variables":{"A":"1","B":"2"}}`)

	out, err := m.Mask(in, []string{"env_variables.*"})
	if err != nil {
		t.Fatal(err)
	}
	env := unmarshal(t, out)["env_variables"].(map[string]any)
	for k, v := range env {
		if v.(string)[:5] != maskPrefix {
			t.Errorf("key %q not masked: %v", k, v)
		}
	}
}

func TestMask_ObjectLeaf(t *testing.T) {
	m := New(testKey)
	in := json.RawMessage(`{"gcs_sink_credential":{"type":"service_account","key":"abc"}}`)

	out, err := m.Mask(in, []string{"gcs_sink_credential"})
	if err != nil {
		t.Fatal(err)
	}
	got := unmarshal(t, out)
	s, ok := got["gcs_sink_credential"].(string)
	if !ok || s[:5] != maskPrefix {
		t.Errorf("object leaf not masked to string: %v", got["gcs_sink_credential"])
	}
}

func TestMask_UnresolvedPathSkipped(t *testing.T) {
	m := New(testKey)
	in := json.RawMessage(`{"env_variables":{"A":"1"}}`)

	out, err := m.Mask(in, []string{"does.not.exist", "env_variables.MISSING"})
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != `{"env_variables":{"A":"1"}}` {
		t.Errorf("payload changed: %s", out)
	}
}

func TestMask_NonStringLeafTypes(t *testing.T) {
	m := New(testKey)
	in := json.RawMessage(`{"token":12345,"flag":true}`)

	out, err := m.Mask(in, []string{"token", "flag"})
	if err != nil {
		t.Fatal(err)
	}
	got := unmarshal(t, out)
	if s, ok := got["token"].(string); !ok || s[:5] != maskPrefix {
		t.Errorf("numeric leaf not masked: %v", got["token"])
	}
	if s, ok := got["flag"].(string); !ok || s[:5] != maskPrefix {
		t.Errorf("bool leaf not masked: %v", got["flag"])
	}
}

func TestFingerprint_DeterministicAndSensitive(t *testing.T) {
	m := New(testKey)

	fp1 := m.fingerprint("hunter2")
	fp2 := m.fingerprint("hunter2")
	if fp1 != fp2 {
		t.Errorf("fingerprint not deterministic: %q != %q", fp1, fp2)
	}
	if len(fp1) != fingerprintLen {
		t.Errorf("fingerprint length = %d, want %d", len(fp1), fingerprintLen)
	}
	if m.fingerprint("s3cr3t!") == fp1 {
		t.Errorf("different values produced same fingerprint")
	}

	// Different key -> different fingerprint.
	if New([]byte("other-key")).fingerprint("hunter2") == fp1 {
		t.Errorf("different key produced same fingerprint")
	}
}

func TestMask_EmptyInputs(t *testing.T) {
	m := New(testKey)
	if out, err := m.Mask(nil, []string{"a"}); err != nil || out != nil {
		t.Errorf("nil payload: out=%v err=%v", out, err)
	}
	in := json.RawMessage(`{"a":"b"}`)
	if out, err := m.Mask(in, nil); err != nil || string(out) != string(in) {
		t.Errorf("no paths: out=%s err=%v", out, err)
	}
}

func TestRestore_MaskedInputRestoresStored(t *testing.T) {
	m := New(testKey)
	incoming := json.RawMessage(`{"env_variables":{"PWD":"****-abcd1234","REPLICAS":"5"}}`)
	stored := json.RawMessage(`{"env_variables":{"PWD":"hunter2","REPLICAS":"3"}}`)

	out, err := m.Restore(incoming, stored, []string{"env_variables.PWD"})
	if err != nil {
		t.Fatal(err)
	}
	env := unmarshal(t, out)["env_variables"].(map[string]any)
	if env["PWD"] != "hunter2" {
		t.Errorf("masked value not restored: %v", env["PWD"])
	}
	if env["REPLICAS"] != "5" {
		t.Errorf("non-sensitive incoming value changed: %v", env["REPLICAS"])
	}
}

func TestRestore_RealInputPersists(t *testing.T) {
	m := New(testKey)
	incoming := json.RawMessage(`{"env_variables":{"PWD":"newsecret"}}`)
	stored := json.RawMessage(`{"env_variables":{"PWD":"hunter2"}}`)

	out, err := m.Restore(incoming, stored, []string{"env_variables.PWD"})
	if err != nil {
		t.Fatal(err)
	}
	env := unmarshal(t, out)["env_variables"].(map[string]any)
	if env["PWD"] != "newsecret" {
		t.Errorf("real value not persisted: %v", env["PWD"])
	}
}

func TestRestore_CreateDropsMaskedInput(t *testing.T) {
	m := New(testKey)
	incoming := json.RawMessage(`{"env_variables":{"PWD":"****-abcd1234","REPLICAS":"5"}}`)

	out, err := m.Restore(incoming, nil, []string{"env_variables.PWD"})
	if err != nil {
		t.Fatal(err)
	}
	env := unmarshal(t, out)["env_variables"].(map[string]any)
	if _, ok := env["PWD"]; ok {
		t.Errorf("masked value with nothing to restore should be dropped, got: %v", env["PWD"])
	}
	if env["REPLICAS"] != "5" {
		t.Errorf("non-sensitive incoming value changed: %v", env["REPLICAS"])
	}
}

func TestRestore_Wildcard(t *testing.T) {
	m := New(testKey)
	incoming := json.RawMessage(`{"env":{"A":"****-1111aaaa","B":"newB"}}`)
	stored := json.RawMessage(`{"env":{"A":"realA","B":"oldB"}}`)

	out, err := m.Restore(incoming, stored, []string{"env.*"})
	if err != nil {
		t.Fatal(err)
	}
	env := unmarshal(t, out)["env"].(map[string]any)
	if env["A"] != "realA" {
		t.Errorf("masked A not restored: %v", env["A"])
	}
	if env["B"] != "newB" {
		t.Errorf("real B changed: %v", env["B"])
	}
}

func TestValidatePaths(t *testing.T) {
	valid := []string{"a", "a.b.c", "env_variables.*", "a.b.*"}
	if err := ValidatePaths(valid); err != nil {
		t.Errorf("valid paths rejected: %v", err)
	}

	invalid := [][]string{
		{""},
		{"a..b"},
		{"a.*.b"},
		{"a.b*"},
		{".a"},
		{"a."},
	}
	for _, paths := range invalid {
		if err := ValidatePaths(paths); err == nil {
			t.Errorf("invalid paths accepted: %v", paths)
		}
	}
}
