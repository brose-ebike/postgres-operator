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

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"

	apiV1 "github.com/brose-ebike/postgres-operator/api/v1"
)

// s3Destination mirrors cloud-pgdumper's S3Destination
// (src/pgdumper/archive/s3.py): a configured bucket/prefix, auto-created on
// construction, with objects addressed as "<prefix>/<target>".
type s3Destination struct {
	client *minio.Client
	bucket string
	prefix string
}

func newS3Destination(ctx context.Context, cfg *apiV1.PgBackupStorageS3, accessKey string, secretKey string) (*s3Destination, error) {
	client, err := minio.New(cfg.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
		Secure: cfg.IsSecure(),
	})
	if err != nil {
		return nil, err
	}

	exists, err := client.BucketExists(ctx, cfg.Bucket)
	if err != nil {
		return nil, err
	}
	if !exists {
		if err := client.MakeBucket(ctx, cfg.Bucket, minio.MakeBucketOptions{}); err != nil {
			return nil, err
		}
	}

	return &s3Destination{
		client: client,
		bucket: cfg.Bucket,
		prefix: strings.Trim(cfg.Prefix, "/"),
	}, nil
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
