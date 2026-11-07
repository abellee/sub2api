package mediahub

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

func settingsFixture(root string) Settings {
	return Settings{MediadBaseURL: "http://127.0.0.1:8090", ManagementToken: "management-private", RelaySecret: "relay-private", CallbackSecret: "callback-private", StorePath: filepath.Join(root, "bindings.json"), ForwardSeconds: 900, ManagementSeconds: 30}
}
func TestSettingsPreserveSecretsAndRestart(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "settings.json")
	defaults := settingsFixture(root)
	store, err := OpenSettings(path, defaults)
	if err != nil {
		t.Fatal(err)
	}
	next := store.Desired()
	next.ManagementToken = ""
	next.RelaySecret = ""
	next.CallbackSecret = ""
	next.ForwardSeconds = 300
	if err = store.Update(next); err != nil {
		t.Fatal(err)
	}
	if store.Active().ForwardSeconds != 900 {
		t.Fatal("active changed before restart")
	}
	if store.Desired().CallbackSecret != defaults.CallbackSecret {
		t.Fatal("empty field erased secret")
	}
	b, _ := json.Marshal(store.View())
	if strings.Contains(string(b), "-private") {
		t.Fatal("secrets exposed")
	}
	reopened, err := OpenSettings(path, defaults)
	if err != nil || reopened.Active().ForwardSeconds != 300 {
		t.Fatal("persistence failed", err)
	}
	next = reopened.Desired()
	next.Enabled = true
	next.CallbackSecret = ""
	next.RelaySecret = ""
	next.MediadBaseURL = ""
	if err = reopened.Update(next); err == nil {
		t.Fatal("enabled relay accepted without connection")
	}
	next.Enabled = false
	next.MediadBaseURL = "https://user:secret@example.com"
	if reopened.Update(next) == nil {
		t.Fatal("URL embedded credentials accepted")
	}
}

func TestBindingFailedWriteKeepsMemory(t *testing.T) {
	root := t.TempDir()
	store, err := NewBindingStore(filepath.Join(root, "binding.json"))
	if err != nil {
		t.Fatal(err)
	}
	original := Binding{GroupID: 1, SchemaID: "one", Enabled: true}
	if err = store.Put(original); err != nil {
		t.Fatal(err)
	}
	store.path = filepath.Join(root, "binding.json", "blocked.json")
	if store.Put(Binding{GroupID: 1, SchemaID: "two"}) == nil {
		t.Fatal("write should fail")
	}
	if got, _ := store.Get(1); got != original {
		t.Fatal("failed put mutated memory")
	}
	if _, err = store.Delete(1); err == nil {
		t.Fatal("delete should fail")
	}
	if got, _ := store.Get(1); got != original {
		t.Fatal("failed delete mutated memory")
	}
}
