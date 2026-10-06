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

// Package dumper invokes pg_dump against a resolved Postgres connection,
// mirroring cloud-pgdumper's pgservice/dumper_service.py: connection
// parameters are passed via environment variables, not CLI flags.
package dumper

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"

	apiV1 "github.com/brose-ebike/postgres-operator/api/v1"
)

// Options configures a single pg_dump invocation.
type Options struct {
	Host     string
	Port     int
	Username string
	Password string
	SSLMode  string
	Database string

	Format           apiV1.PgBackupDumpFormat
	CompressionLevel int32

	// OutputPath is the file (or, for Format=directory, the directory)
	// pg_dump is asked to write to.
	OutputPath string
}

// DumpError wraps a non-zero pg_dump exit, following this repo's typed
// domain error convention (pkg/oebc_errors).
type DumpError struct {
	Database string
	Reason   string
	err      error
}

func NewDumpError(database string, reason string, err error) *DumpError {
	return &DumpError{Database: database, Reason: reason, err: err}
}

func (e *DumpError) Error() string {
	return fmt.Sprintf("pg_dump failed for database %q: %s", e.Database, e.Reason)
}

func (e *DumpError) Unwrap() error {
	return e.err
}

func formatFlag(f apiV1.PgBackupDumpFormat) string {
	switch f {
	case apiV1.PgBackupDumpFormatDirectory:
		return "directory"
	case apiV1.PgBackupDumpFormatTar:
		return "tar"
	case apiV1.PgBackupDumpFormatPlain:
		return "plain"
	default:
		return "custom"
	}
}

// Dump shells out to pg_dump with the given options and returns the path to
// the single-file artifact ready for encryption/upload. For every format
// except "directory" this is simply opts.OutputPath; pg_dump's "directory"
// format instead writes a directory of files, which is tarred and gzipped
// into "<OutputPath>.tar.gz" so storage.Destination (which only stores
// single objects) can still handle it uniformly.
func Dump(ctx context.Context, opts Options) (string, error) {
	outputPath := opts.OutputPath
	args := []string{
		"--no-password",
		fmt.Sprintf("--compress=%d", opts.CompressionLevel),
		"--format=" + formatFlag(opts.Format),
		"--file=" + outputPath,
	}

	cmd := exec.CommandContext(ctx, "pg_dump", args...)
	cmd.Env = append(os.Environ(),
		"PGHOST="+opts.Host,
		"PGPORT="+strconv.Itoa(opts.Port),
		"PGUSER="+opts.Username,
		"PGPASSWORD="+opts.Password,
		"PGDATABASE="+opts.Database,
	)
	if opts.SSLMode != "" {
		cmd.Env = append(cmd.Env, "PGSSLMODE="+opts.SSLMode)
	}

	var stderr strings.Builder
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		reason := strings.TrimSpace(stderr.String())
		if reason == "" {
			reason = "unknown error"
		}
		return "", NewDumpError(opts.Database, reason, err)
	}

	if opts.Format != apiV1.PgBackupDumpFormatDirectory {
		return outputPath, nil
	}

	artifactPath := outputPath + ".tar.gz"
	if err := tarGzipDirectory(outputPath, artifactPath); err != nil {
		return "", NewDumpError(opts.Database, "failed to archive directory-format dump: "+err.Error(), err)
	}
	return artifactPath, nil
}
