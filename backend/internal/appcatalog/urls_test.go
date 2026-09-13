package appcatalog

import "testing"

func TestNormalizeSourceURLsRequiresOne(t *testing.T) {
	if _, _, err := NormalizeSourceURLs("", ""); err == nil {
		t.Fatal("expected error when both URLs are empty")
	}
}

func TestNormalizeGitHubURL(t *testing.T) {
	got, err := normalizeGitHubURL("git@github.com:acme/demo.git")
	if err != nil {
		t.Fatal(err)
	}
	if got != "https://github.com/acme/demo" {
		t.Fatalf("got %s", got)
	}
}


