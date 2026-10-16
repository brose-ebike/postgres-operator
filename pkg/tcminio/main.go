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

package tcminio

import (
	"context"
	"strconv"
	"time"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

// MinioContainer represents the minio container type used in the module
type MinioContainer struct {
	container testcontainers.Container
	accessKey string
	secretKey string
	bucket    string
}

type MinioRequest struct {
	accessKey string
	secretKey string
	bucket    string
}

type MinioContainerOption func(tcReq *testcontainers.ContainerRequest, mReq *MinioRequest)

func WithCredentials(accessKey string, secretKey string) MinioContainerOption {
	return func(tcReq *testcontainers.ContainerRequest, mReq *MinioRequest) {
		tcReq.Env["MINIO_ROOT_USER"] = accessKey
		tcReq.Env["MINIO_ROOT_PASSWORD"] = secretKey
		mReq.accessKey = accessKey
		mReq.secretKey = secretKey
	}
}

// WithBucket records the bucket name tests intend to use. Bucket creation
// itself is left to the storage.Destination under test (which auto-creates
// its bucket on construction), so this option only carries the name through
// for assertions/accessors.
func WithBucket(bucket string) MinioContainerOption {
	return func(tcReq *testcontainers.ContainerRequest, mReq *MinioRequest) {
		mReq.bucket = bucket
	}
}

// SetupMinio creates an instance of the minio container type
func SetupMinio(ctx context.Context, opts ...MinioContainerOption) (*MinioContainer, error) {
	tcReq := testcontainers.ContainerRequest{
		// MinIO pulled minio/minio off Docker Hub entirely and locked
		// quay.io/minio/minio behind auth too - both now reject anonymous
		// pulls for every tag. cgr.dev/chainguard/minio is Chainguard's
		// actively-maintained mirror: same entrypoint/args/health-check
		// contract as upstream, and still anonymously pullable.
		Image:        "cgr.dev/chainguard/minio:latest",
		Env:          map[string]string{},
		ExposedPorts: []string{"9000/tcp"},
		Cmd:          []string{"server", "/data"},
		WaitingFor: wait.
			ForHTTP("/minio/health/ready").
			WithPort("9000/tcp").
			WithStartupTimeout(30 * time.Second),
	}
	mReq := MinioRequest{}

	for _, opt := range opts {
		opt(&tcReq, &mReq)
	}

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: tcReq,
		Started:          true,
	})
	if err != nil {
		return nil, err
	}

	return &MinioContainer{
		container: container,
		accessKey: mReq.accessKey,
		secretKey: mReq.secretKey,
		bucket:    mReq.bucket,
	}, nil
}

func (mc *MinioContainer) Terminate(ctx context.Context) error {
	if mc.container == nil {
		return nil
	}
	return mc.container.Terminate(ctx)
}

func (mc *MinioContainer) Hostname(ctx context.Context) (string, error) {
	return mc.container.Host(ctx)
}

func (mc *MinioContainer) Port(ctx context.Context) (int, error) {
	containerPort, err := mc.container.MappedPort(ctx, "9000/tcp")
	if err != nil {
		return 0, err
	}
	return int(containerPort.Num()), nil
}

// Endpoint returns "host:port" suitable for minio.New's endpoint argument.
func (mc *MinioContainer) Endpoint(ctx context.Context) (string, error) {
	host, err := mc.Hostname(ctx)
	if err != nil {
		return "", err
	}
	port, err := mc.Port(ctx)
	if err != nil {
		return "", err
	}
	return host + ":" + strconv.Itoa(port), nil
}

func (mc *MinioContainer) AccessKey() string {
	return mc.accessKey
}

func (mc *MinioContainer) SecretKey() string {
	return mc.secretKey
}

func (mc *MinioContainer) Bucket() string {
	return mc.bucket
}
