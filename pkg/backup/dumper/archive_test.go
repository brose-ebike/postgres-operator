/*
Copyright 2026 Yamaha Motor eBike Systems GmbH.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package dumper

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestTarGzipDirectory(t *testing.T) {
	srcDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(srcDir, "toc.dat"), []byte("fake toc"), 0o600); err != nil {
		t.Fatalf("unable to seed toc.dat: %v", err)
	}
	if err := os.Mkdir(filepath.Join(srcDir, "data"), 0o700); err != nil {
		t.Fatalf("unable to seed data dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "data", "1.dat"), []byte("fake table data"), 0o600); err != nil {
		t.Fatalf("unable to seed data/1.dat: %v", err)
	}

	dstPath := filepath.Join(t.TempDir(), "dump.tar.gz")
	if err := tarGzipDirectory(srcDir, dstPath); err != nil {
		t.Fatalf("tarGzipDirectory failed: %v", err)
	}

	f, err := os.Open(dstPath)
	if err != nil {
		t.Fatalf("unable to open archive: %v", err)
	}
	defer f.Close()

	gzReader, err := gzip.NewReader(f)
	if err != nil {
		t.Fatalf("archive is not valid gzip: %v", err)
	}
	defer gzReader.Close()

	tarReader := tar.NewReader(gzReader)
	found := map[string]string{}
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("unable to read tar entry: %v", err)
		}
		if header.Typeflag == tar.TypeDir {
			continue
		}
		contents, err := io.ReadAll(tarReader)
		if err != nil {
			t.Fatalf("unable to read contents of %s: %v", header.Name, err)
		}
		found[header.Name] = string(contents)
	}

	if found["toc.dat"] != "fake toc" {
		t.Errorf("expected toc.dat contents to round-trip, got %q", found["toc.dat"])
	}
	if found[filepath.Join("data", "1.dat")] != "fake table data" {
		t.Errorf("expected data/1.dat contents to round-trip, got %q", found[filepath.Join("data", "1.dat")])
	}
}

// failAfterWriter fails every Write once maxBytes have passed through it,
// simulating a late write failure (e.g. the workspace volume filling up
// right at the final flush) during tar/gzip's Close(). It lets a test
// inject a failure at the exact point the previous bare-defer
// implementation would have silently swallowed it.
type failAfterWriter struct {
	maxBytes int
	written  int
}

func (f *failAfterWriter) Write(p []byte) (int, error) {
	if f.written >= f.maxBytes {
		return 0, fmt.Errorf("simulated write failure: workspace full")
	}
	remaining := f.maxBytes - f.written
	n := len(p)
	if n > remaining {
		n = remaining
	}
	f.written += n
	if n < len(p) {
		return n, fmt.Errorf("simulated write failure: workspace full")
	}
	return n, nil
}

func TestTarGzip_SurfacesLateWriteFailureInsteadOfSilentlyTruncating(t *testing.T) {
	srcDir := t.TempDir()
	// A file large enough that gzip compression plus tar's own headers
	// guarantee several hundred bytes pass through the writer before the
	// final Close() flush - the failure point below must land inside
	// that flush, not the initial header/data writes, to actually prove
	// Close()'s own error is the one being surfaced.
	if err := os.WriteFile(filepath.Join(srcDir, "toc.dat"), make([]byte, 4096), 0o600); err != nil {
		t.Fatalf("unable to seed toc.dat: %v", err)
	}

	w := &failAfterWriter{maxBytes: 16}
	err := tarGzip(w, srcDir)
	if err == nil {
		t.Fatal("expected tarGzip to surface the simulated write failure, got nil error")
	}
}
