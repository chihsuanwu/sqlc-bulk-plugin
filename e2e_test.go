//go:build e2e

package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestE2E(t *testing.T) {
	// Ensure sqlc is available
	if _, err := exec.LookPath("sqlc"); err != nil {
		t.Skip("sqlc not installed, skipping e2e test")
	}

	projectDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}

	// Step 1: Build the plugin binary into a temp directory
	tmpDir := t.TempDir()
	pluginBin := filepath.Join(tmpDir, "sqlc-bulk-plugin")

	build := exec.Command("go", "build", "-o", pluginBin, ".")
	build.Dir = projectDir
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("failed to build plugin: %v\n%s", err, out)
	}

	// Step 2: Write a temp sqlc.yaml in project dir pointing to the built binary
	sqlcYaml := `version: "2"
plugins:
  - name: bulk
    process:
      cmd: ` + pluginBin + `
sql:
  - schema: testdata/schema.sql
    queries: testdata/query.sql
    engine: postgresql
    gen:
      go:
        package: db
        out: gen
        sql_package: pgx/v5
        emit_interface: true
    codegen:
      - plugin: bulk
        out: gen
        options:
          package: db
`
	tmpYaml := filepath.Join(projectDir, "sqlc_e2e.yaml")
	if err := os.WriteFile(tmpYaml, []byte(sqlcYaml), 0644); err != nil {
		t.Fatalf("failed to write temp sqlc.yaml: %v", err)
	}
	defer os.Remove(tmpYaml)

	// Step 3: Clean gen/ directory (keep go.mod/go.sum if any)
	genDir := filepath.Join(projectDir, "gen")
	entries, _ := os.ReadDir(genDir)
	for _, e := range entries {
		if filepath.Ext(e.Name()) == ".go" {
			os.Remove(filepath.Join(genDir, e.Name()))
		}
	}

	// Step 4: Run sqlc generate
	generate := exec.Command("sqlc", "generate", "-f", tmpYaml)
	generate.Dir = projectDir
	if out, err := generate.CombinedOutput(); err != nil {
		t.Fatalf("sqlc generate failed: %v\n%s", err, out)
	}

	// Step 5: Verify generated code compiles
	buildGen := exec.Command("go", "build", "./gen")
	buildGen.Dir = projectDir
	if out, err := buildGen.CombinedOutput(); err != nil {
		t.Fatalf("generated code failed to compile: %v\n%s", err, out)
	}

	// Step 6: Compare bulk.go against golden file
	gotPath := filepath.Join(genDir, "bulk.go")
	goldenPath := filepath.Join(projectDir, "testdata", "golden", "e2e_bulk.go.golden")

	got, err := os.ReadFile(gotPath)
	if err != nil {
		t.Fatalf("failed to read generated bulk.go: %v", err)
	}

	if *update {
		if err := os.WriteFile(goldenPath, got, 0644); err != nil {
			t.Fatalf("failed to update golden file: %v", err)
		}
		t.Log("golden file updated")
		return
	}

	want, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("failed to read golden file (run with -update to create): %v", err)
	}

	if string(got) != string(want) {
		t.Errorf("generated bulk.go does not match golden file.\n\nGot:\n%s\n\nWant:\n%s", got, want)
	}
}
