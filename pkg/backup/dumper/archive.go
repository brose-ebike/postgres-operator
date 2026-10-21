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
	"errors"
	"io"
	"os"
	"path/filepath"
)

// tarGzipDirectory writes every file under srcDir into a gzip-compressed
// tar archive at dstPath, with entry names relative to srcDir. Used to turn
// pg_dump's "directory" format output (a directory of files) into the
// single-object artifact storage.Destination expects.
func tarGzipDirectory(srcDir string, dstPath string) (err error) {
	out, err := os.Create(dstPath)
	if err != nil {
		return err
	}
	defer func() {
		err = errors.Join(err, out.Close())
	}()

	return tarGzip(out, srcDir)
}

// tarGzip writes srcDir into w as a gzip-compressed tar stream. Split out
// from tarGzipDirectory so a test can inject a writer that fails partway
// through, proving the Close() error handling below actually works -
// something a destination-file-only test could never exercise.
//
// tarWriter.Close() and gzWriter.Close() both flush buffered trailer/footer
// bytes (tar's end-of-archive padding, gzip's CRC/size footer). A bare
// `defer tarWriter.Close()` / `defer gzWriter.Close()` - the previous
// implementation - silently drops a late flush failure (e.g. the
// emptyDir workspace filling up right at the end, plausible here since it
// holds the directory-format dump and its tar.gz copy at once): the
// function would still return nil, reporting success with a truncated,
// corrupt archive already on disk. Capturing and joining every error here
// instead means a flush failure actually surfaces as a Dump error, not a
// backup that silently can't be restored.
func tarGzip(w io.Writer, srcDir string) error {
	gzWriter := gzip.NewWriter(w)
	tarWriter := tar.NewWriter(gzWriter)

	walkErr := filepath.Walk(srcDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		relPath, err := filepath.Rel(srcDir, path)
		if err != nil {
			return err
		}
		if relPath == "." {
			return nil
		}

		header, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return err
		}
		header.Name = relPath

		if err := tarWriter.WriteHeader(header); err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}

		file, err := os.Open(path)
		if err != nil {
			return err
		}
		defer file.Close()

		_, err = io.Copy(tarWriter, file)
		return err
	})

	// Close tarWriter before gzWriter regardless of walkErr: tar's Close()
	// flushes its end-of-archive padding into gzWriter, which gzWriter's
	// own Close() then needs to compress before writing its footer -
	// closing out of order (or skipping one) produces a gzip stream
	// decompressors reject or silently truncate.
	return errors.Join(walkErr, tarWriter.Close(), gzWriter.Close())
}
