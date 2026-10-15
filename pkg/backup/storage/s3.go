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

package storage

import (
	"context"
	"strings"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// s3Destination mirrors cloud-pgdumper's S3Destination
// (src/pgdumper/archive/s3.py): a configured bucket/prefix, auto-created on
// construction, with objects addressed as "<prefix>/<target>".
type s3Destination struct {
	client *minio.Client
	bucket string
	prefix string
}

func newS3Destination(ctx context.Context, endpoint string, bucket string, prefix string, secure bool, accessKey string, secretKey string) (*s3Destination, error) {
	client, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
		Secure: secure,
	})
	if err != nil {
		return nil, err
	}

	exists, err := bucketExistsWithRetry(ctx, client, bucket)
	if err != nil {
		return nil, err
	}
	if !exists {
		if err := client.MakeBucket(ctx, bucket, minio.MakeBucketOptions{}); err != nil {
			return nil, err
		}
	}

	return &s3Destination{
		client: client,
		bucket: bucket,
		// Unlike cloud-pgdumper, an empty prefix is NOT defaulted to
		// "pgbackups/": the worker's target keys already carry full
		// "<namespace>/<database>/<instance>.dump" structure before Store
		// is ever called (see pkg/backup/worker), so there's no shared-
		// bucket collision risk a default prefix would be guarding
		// against here. prefix stays purely opt-in, for callers who want
		// an additional namespace layer on top.
		prefix: strings.Trim(prefix, "/"),
	}, nil
}

// bucketExistsWithRetry retries BucketExists a handful of times with a
// short backoff. A storage backend can report its health check ready
// slightly before it can actually serve bucket operations (MinIO: "Server
// not initialized yet, please try again.", HTTP 503) - this covers that
// startup window for both the real worker and tests against a fresh
// container, without masking a genuinely broken endpoint/credentials.
func bucketExistsWithRetry(ctx context.Context, client *minio.Client, bucket string) (exists bool, err error) {
	const attempts = 5
	backoff := 250 * time.Millisecond
	for i := 0; i < attempts; i++ {
		exists, err = client.BucketExists(ctx, bucket)
		if err == nil {
			return exists, nil
		}
		if i == attempts-1 {
			break
		}
		select {
		case <-ctx.Done():
			return false, ctx.Err()
		case <-time.After(backoff):
		}
		backoff *= 2
	}
	return false, err
}

// objectKey joins the configured prefix and target exactly like
// cloud-pgdumper: prefix + "/" + target when a prefix is set, else the bare
// target.
func (d *s3Destination) objectKey(target string) string {
	if d.prefix == "" {
		return target
	}
	return d.prefix + "/" + target
}

func (d *s3Destination) Store(ctx context.Context, sourcePath string, target string) error {
	_, err := d.client.FPutObject(ctx, d.bucket, d.objectKey(target), sourcePath, minio.PutObjectOptions{})
	return err
}

func (d *s3Destination) List(ctx context.Context) ([]string, error) {
	listPrefix := d.prefix
	if listPrefix != "" {
		listPrefix += "/"
	}

	var names []string
	for obj := range d.client.ListObjects(ctx, d.bucket, minio.ListObjectsOptions{Prefix: listPrefix, Recursive: true}) {
		if obj.Err != nil {
			return nil, obj.Err
		}
		names = append(names, strings.TrimPrefix(obj.Key, listPrefix))
	}
	return names, nil
}

func (d *s3Destination) Delete(ctx context.Context, target string) error {
	return d.client.RemoveObject(ctx, d.bucket, d.objectKey(target), minio.RemoveObjectOptions{})
}
