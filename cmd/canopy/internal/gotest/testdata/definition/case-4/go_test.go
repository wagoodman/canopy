package case_4

import (
	"encoding/json"
	"os"
	"path"
	"testing"
)

func TestNewZip64FileManifest(t *testing.T) {
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}

	sourceDirPath := path.Join(cwd, "test-fixtures", "zip-source")
	archiveFilePath := setupZipFileTest(t, sourceDirPath, true)

	actual, err := NewZipFileManifest(archiveFilePath)
	if err != nil {
		t.Fatalf("unable to extract from unzip archive: %+v", err)
	}

	if len(expectedZipArchiveEntries) != len(actual) {
		t.Fatalf("mismatched manifest: %d != %d", len(actual), len(expectedZipArchiveEntries))
	}

	// the important part about this test is that this expectedZipArchiveEntries var is out of scope when looking at the AST
	for _, e := range expectedZipArchiveEntries {
		_, ok := actual[e]
		if !ok {
			t.Errorf("missing path: %s", e)
		}
	}

	if t.Failed() {
		b, err := json.MarshalIndent(actual, "", "  ")
		if err != nil {
			t.Fatalf("can't show results: %+v", err)
		}

		t.Errorf("full result: %s", string(b))
	}
}

type ZipArchiveEntry struct {
	Path string
}

func setupZipFileTest(t *testing.T, sourceDirPath string, zip64 bool) string {
	return ""
}

func NewZipFileManifest(archiveFilePath string) (map[string]ZipArchiveEntry, error) {
	return nil, nil
}
