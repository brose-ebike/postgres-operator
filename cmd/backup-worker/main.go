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

// Command backup-worker is the CLI the PgBackupPolicy-owned CronJobs invoke
// (via a command override against the same image the manager runs as). It
// is a thin shim: all real logic lives in pkg/backup/worker, which is
// exercised directly (as a function call, not a subprocess) by that
// package's own tests and the e2e suite.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	postgresv1 "github.com/brose-ebike/postgres-operator/api/v1"
	"github.com/brose-ebike/postgres-operator/pkg/backup/worker"
)

const defaultWorkspaceDir = "/app/workspace"

var scheme = runtime.NewScheme()

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(postgresv1.AddToScheme(scheme))
}

func main() {
	ctrl.SetLogger(zap.New())

	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	subcommand := os.Args[1]

	fs := flag.NewFlagSet(subcommand, flag.ExitOnError)
	policyFlag := fs.String("policy", "", "namespace/name of the PgBackupPolicy")
	workspaceDir := fs.String("workspace-dir", defaultWorkspaceDir, "scratch directory for dump artifacts (dump subcommand only)")
	if err := fs.Parse(os.Args[2:]); err != nil {
		os.Exit(2)
	}

	policyRef, err := parsePolicyRef(*policyFlag)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}

	c, err := newClient()
	if err != nil {
		fmt.Fprintln(os.Stderr, "unable to build Kubernetes client:", err)
		os.Exit(1)
	}

	ctx := context.Background()
	switch subcommand {
	case "dump":
		err = worker.RunDump(ctx, c, policyRef, *workspaceDir)
	case "cleanup":
		err = worker.RunCleanup(ctx, c, policyRef)
	default:
		fmt.Fprintf(os.Stderr, "unknown subcommand %q\n\n", subcommand)
		usage()
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage: backup-worker <dump|cleanup> --policy=namespace/name [--workspace-dir=/app/workspace]")
}

func parsePolicyRef(value string) (types.NamespacedName, error) {
	namespace, name, found := strings.Cut(value, "/")
	if !found || namespace == "" || name == "" {
		return types.NamespacedName{}, fmt.Errorf("invalid --policy value %q, expected namespace/name", value)
	}
	return types.NamespacedName{Namespace: namespace, Name: name}, nil
}

// newClient uses ctrl.GetConfig() (not GetConfigOrDie(), which log.Fatals
// internally) so this binary can report a clean error and non-zero exit
// code instead. Works both in-cluster and against a local kubeconfig, same
// as cmd/manager.
func newClient() (client.Client, error) {
	cfg, err := ctrl.GetConfig()
	if err != nil {
		return nil, err
	}
	return client.New(cfg, client.Options{Scheme: scheme})
}
