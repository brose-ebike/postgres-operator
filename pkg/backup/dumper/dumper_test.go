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
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	apiV1 "github.com/brose-ebike/postgres-operator/api/v1"
	"github.com/brose-ebike/postgres-operator/pkg/tcpostgres"
)

func setupTestOptions(t *testing.T, ctx context.Context) (Options, func()) {
	t.Helper()

	container, err := tcpostgres.SetupPostgres(ctx, tcpostgres.WithInitialDatabase("pgtest", "pgtest", "pgtest"))
	if err != nil {
		t.Fatalf("unable to setup postgres container: %v", err)
	}

	host, err := container.Hostname(ctx)
	if err != nil {
		t.Fatalf("unable to resolve container hostname: %v", err)
	}
	port, err := container.Port(ctx)
	if err != nil {
		t.Fatalf("unable to resolve container port: %v", err)
	}

	cleanup := func() {
		if err := container.Terminate(ctx); err != nil {
			t.Errorf("unable to terminate postgres container: %v", err)
		}
	}

	return Options{
		Host:     host,
		Port:     port,
		Username: container.Username(),
		Password: container.Password(),
		Database: container.Database(),
		SSLMode:  "disable",
	}, cleanup
}

func TestDump_AllFormats(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	baseOpts, cleanup := setupTestOptions(t, ctx)
	defer cleanup()

	cases := []struct {
		name   string
		format apiV1.PgBackupDumpFormat
	}{
		{"plain", apiV1.PgBackupDumpFormatPlain},
		{"custom", apiV1.PgBackupDumpFormatCustom},
		{"tar", apiV1.PgBackupDumpFormatTar},
		{"directory", apiV1.PgBackupDumpFormatDirectory},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			opts := baseOpts
			opts.Format = tc.format
			opts.CompressionLevel = 6
			opts.OutputPath = filepath.Join(t.TempDir(), "dump-"+tc.name)

			artifactPath, err := Dump(ctx, opts)
			if err != nil {
				t.Fatalf("Dump failed for format %s: %v", tc.format, err)
			}

			if tc.format == apiV1.PgBackupDumpFormatDirectory {
				if artifactPath != opts.OutputPath+".tar.gz" {
					t.Fatalf("expected directory format to produce %q, got %q", opts.OutputPath+".tar.gz", artifactPath)
				}
			} else if artifactPath != opts.OutputPath {
				t.Fatalf("expected artifact path %q, got %q", opts.OutputPath, artifactPath)
			}

			info, err := os.Stat(artifactPath)
			if err != nil {
				t.Fatalf("expected artifact to exist at %q: %v", artifactPath, err)
			}
			if info.Size() == 0 {
				t.Fatalf("expected non-empty artifact at %q", artifactPath)
			}
		})
	}
}

func TestDump_BadConnection(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	opts := Options{
		Host:             "127.0.0.1",
		Port:             1, // nothing listens here
		Username:         "nobody",
		Password:         "wrong",
		Database:         "nope",
		SSLMode:          "disable",
		Format:           apiV1.PgBackupDumpFormatCustom,
		CompressionLevel: 6,
		OutputPath:       filepath.Join(t.TempDir(), "dump"),
	}

	_, err := Dump(ctx, opts)
	if err == nil {
		t.Fatal("expected Dump against an unreachable server to fail")
	}
	var dumpErr *DumpError
	if !errors.As(err, &dumpErr) {
		t.Fatalf("expected a *DumpError, got %T: %v", err, err)
	}
}
