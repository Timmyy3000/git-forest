package state

import "testing"

func TestSaveReplacesExistingStore(t *testing.T) {
	root := t.TempDir()
	store := NewStore(root)
	store.DefaultBase = "main"
	if err := Save(root, store); err != nil {
		t.Fatal(err)
	}

	store.DefaultBase = "trunk"
	if err := Save(root, store); err != nil {
		t.Fatal(err)
	}
	loaded, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.DefaultBase != "trunk" {
		t.Fatalf("default base = %q, want trunk", loaded.DefaultBase)
	}
}
