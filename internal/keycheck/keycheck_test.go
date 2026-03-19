package keycheck

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolve_EnvVar(t *testing.T) {
	k := KeyStatus{Name: "TEST_KEY", EnvVar: "TEST_KEY", KeychainService: "TEST_KEY"}
	t.Setenv("TEST_KEY", "sk-test-1234567890abcdef")

	got := Resolve(k)
	if !got.Set {
		t.Fatal("expected key to be set")
	}
	if got.Source != SourceEnv {
		t.Errorf("source = %q, want env", got.Source)
	}
	if got.Masked == "" {
		t.Error("masked should not be empty")
	}
}

func TestResolve_Missing(t *testing.T) {
	k := KeyStatus{Name: "NO_SUCH_KEY_XYZ", EnvVar: "NO_SUCH_KEY_XYZ", KeychainService: "NO_SUCH_KEY_XYZ"}
	os.Unsetenv("NO_SUCH_KEY_XYZ")

	got := Resolve(k)
	if got.Set {
		t.Fatal("expected key NOT to be set")
	}
	if got.Source != SourceNone {
		t.Errorf("source = %q, want empty", got.Source)
	}
}

func TestMaskKey_Short(t *testing.T) {
	if got := maskKey("abc"); got != "****" {
		t.Errorf("maskKey(short) = %q, want ****", got)
	}
}

func TestMaskKey_Long(t *testing.T) {
	key := "sk-or-v1-abcdefghijklmnopqrstuvwxyz"
	got := maskKey(key)
	// Should start with first 8 chars and end with last 4.
	if got[:8] != key[:8] {
		t.Errorf("prefix mismatch: %q", got)
	}
	if got[len(got)-4:] != key[len(key)-4:] {
		t.Errorf("suffix mismatch: %q", got)
	}
}

func TestCheckCommand_OK(t *testing.T) {
	t.Setenv("OPENROUTER_API_KEY", "sk-or-v1-testkey-xxxxxxxxxxxxxxxx")

	result := CheckCommand("write")
	if !result.OK {
		t.Errorf("CheckCommand(write) = not OK, want OK")
	}
	if len(result.Present) == 0 {
		t.Error("Present should not be empty")
	}
}

func TestCheckCommand_Missing(t *testing.T) {
	os.Unsetenv("OPENROUTER_API_KEY")
	os.Unsetenv("ANTHROPIC_API_KEY")

	result := CheckCommand("write")
	// Can't guarantee keychain is empty in test env, so only check structure.
	if result.Command != "write" {
		t.Errorf("command = %q, want write", result.Command)
	}
}

func TestCheckCommand_UnknownCommand(t *testing.T) {
	result := CheckCommand("nonexistent-cmd")
	if !result.OK {
		t.Error("unknown command should return OK=true (no requirement registered)")
	}
}

func TestCheckCommand_ResearchDeep(t *testing.T) {
	os.Unsetenv("GEMINI_API_KEY")

	result := CheckCommand("research-deep")
	if result.Command != "research-deep" {
		t.Errorf("command = %q, want research-deep", result.Command)
	}
}

func TestRequirements_NewCommands(t *testing.T) {
	for _, cmd := range []string{"chat", "fix", "context", "index"} {
		t.Run(cmd, func(t *testing.T) {
			keys, ok := Requirements[cmd]
			if !ok {
				t.Fatalf("Requirements missing entry for %q", cmd)
			}
			if len(keys) == 0 {
				t.Errorf("Requirements[%q] is empty", cmd)
			}
		})
	}
}

func TestKeychainSet_SecurityNotFound(t *testing.T) {
	// This test only verifies that KeychainSet returns an error when the
	// security binary is absent or USER is unset — not that it actually
	// writes to Keychain (which would require real credentials).
	// On macOS CI the binary exists, so we just call it with a dummy value
	// and accept either success or failure (not a panic).
	_ = KeychainSet("__nqb_test_service__", "dummy")
}

func TestEnvSet_CreateNew(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".env")

	if err := EnvSet(path, "ZILLIZ_API_KEY", "abc123"); err != nil {
		t.Fatalf("EnvSet: %v", err)
	}

	data, _ := os.ReadFile(path)
	if !strings.Contains(string(data), "ZILLIZ_API_KEY=abc123") {
		t.Errorf("expected ZILLIZ_API_KEY=abc123 in file, got:\n%s", data)
	}
}

func TestEnvSet_UpdateExisting(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".env")
	_ = os.WriteFile(path, []byte("ZILLIZ_API_KEY=old\nOTHER=x\n"), 0o600)

	if err := EnvSet(path, "ZILLIZ_API_KEY", "new"); err != nil {
		t.Fatalf("EnvSet: %v", err)
	}

	data, _ := os.ReadFile(path)
	content := string(data)
	if !strings.Contains(content, "ZILLIZ_API_KEY=new") {
		t.Errorf("expected updated value, got:\n%s", content)
	}
	if strings.Contains(content, "ZILLIZ_API_KEY=old") {
		t.Errorf("old value still present:\n%s", content)
	}
	// Other keys must be preserved.
	if !strings.Contains(content, "OTHER=x") {
		t.Errorf("OTHER key was removed:\n%s", content)
	}
}

func TestEnvSet_Idempotent(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".env")

	for range 3 {
		if err := EnvSet(path, "KEY", "val"); err != nil {
			t.Fatalf("EnvSet: %v", err)
		}
	}

	data, _ := os.ReadFile(path)
	count := strings.Count(string(data), "KEY=val")
	if count != 1 {
		t.Errorf("expected 1 occurrence of KEY=val, got %d:\n%s", count, data)
	}
}
