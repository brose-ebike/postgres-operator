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
