// Run from backend. -write pins a clean SDK checkout; default verifies the
// exact build inputs, including when Docker excludes the SDK's Git directory.
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type manifest struct {
	Commit string            `json:"commit"`
	Files  map[string]string `json:"files"`
}

func main() {
	write := flag.Bool("write", false, "pin a clean SDK checkout")
	flag.Parse()
	if err := run(*write); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(write bool) error {
	m := manifest{Files: map[string]string{}}
	const lock = "audit-sdk.lock.json"
	if write {
		status, err := exec.Command("git", "-C", "audit-sdk", "status", "--porcelain").Output()
		if err != nil || len(status) != 0 {
			return fmt.Errorf("SDK must be a clean committed checkout: %v", err)
		}
		sha, err := exec.Command("git", "-C", "audit-sdk", "rev-parse", "HEAD").Output()
		if err != nil {
			return err
		}
		m.Commit = strings.TrimSpace(string(sha))
	} else {
		data, err := os.ReadFile(lock)
		if err != nil {
			return err
		}
		if err := json.Unmarshal(data, &m); err != nil {
			return err
		}
		decoded, err := hex.DecodeString(m.Commit)
		if err != nil || len(decoded) != 20 || len(m.Files) == 0 {
			return fmt.Errorf("invalid SDK manifest")
		}
		if expected := os.Getenv("AUDIT_SDK_COMMIT"); expected != "" && expected != m.Commit {
			return fmt.Errorf("SDK build version differs from manifest")
		}
		if _, err := os.Stat("../.git"); err == nil {
			entry, err := exec.Command("git", "-C", "..", "ls-tree", "HEAD", "--", "backend/audit-sdk").Output()
			if err != nil || strings.TrimSpace(string(entry)) != "160000 commit "+m.Commit+"\tbackend/audit-sdk" {
				return fmt.Errorf("SDK manifest differs from main-site gitlink: %v", err)
			}
		}
	}
	seen := map[string]bool{}
	err := filepath.WalkDir("audit-sdk", func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			if entry.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		rel, err := filepath.Rel("audit-sdk", path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if (strings.HasSuffix(rel, "_test.go") || !strings.HasSuffix(rel, ".go")) && rel != "go.mod" && rel != "go.sum" {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		digest := fmt.Sprintf("%x", sha256.Sum256(data))
		if write {
			m.Files[rel] = digest
		} else if m.Files[rel] != digest {
			return fmt.Errorf("SDK build input differs from pin: %s", rel)
		}
		seen[rel] = true
		return nil
	})
	if err != nil {
		return err
	}
	for path := range m.Files {
		if !seen[path] {
			return fmt.Errorf("missing SDK build input: %s", path)
		}
	}
	if write {
		data, err := json.MarshalIndent(m, "", "  ")
		if err != nil {
			return err
		}
		if err := os.WriteFile(lock, append(data, '\n'), 0644); err != nil {
			return err
		}
	}
	fmt.Println(m.Commit)
	return nil
}
