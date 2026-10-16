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

// Package encryption implements at-rest encryption of backup dumps,
// mirroring cloud-pgdumper's gpg-rsa/gpg-aes schemes but as pure Go via
// github.com/ProtonMail/gopenpgp/v2 instead of shelling out to the gpg
// binary. Output is raw (non-armored) OpenPGP binary, not ASCII-armored
// text, to keep stored artifacts compact.
//
// Both schemes stream the dump through encryption rather than buffering
// the whole plaintext/ciphertext in memory - a real dump can be gigabytes
// against a CronJob pod with a 128Mi memory limit, so buffering both
// copies in RAM reliably OOM-kills on anything but a toy database.
package encryption

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/ProtonMail/go-crypto/openpgp"
	"github.com/ProtonMail/gopenpgp/v2/crypto"

	apiV1 "github.com/brose-ebike/postgres-operator/api/v1"
)

// Encryptor encrypts a dump file into a target file.
type Encryptor interface {
	// Encrypt reads from srcPath and writes the encrypted result to dstPath.
	Encrypt(ctx context.Context, srcPath string, dstPath string) error
}

// NewEncryptor is a factory keyed on the PgBackupPolicy's encryption type.
// keyMaterial is the resolved secret value: an armored public key for
// gpg-rsa, a passphrase for gpg-aes.
func NewEncryptor(encType apiV1.PgBackupEncryptionType, keyMaterial string) (Encryptor, error) {
	switch encType {
	case apiV1.PgBackupEncryptionTypeGPGRSA:
		return &rsaEncryptor{publicKeyArmored: keyMaterial}, nil
	case apiV1.PgBackupEncryptionTypeGPGAES:
		return &aesEncryptor{passphrase: keyMaterial}, nil
	default:
		return nil, fmt.Errorf("unsupported encryption type %q", encType)
	}
}

// streamEncrypt copies srcPath into the WriteCloser open returns for
// dstPath, without ever holding the full plaintext or ciphertext in
// memory - io.Copy uses a small fixed-size internal buffer regardless of
// file size. Both the encoder's Close() (which flushes the OpenPGP
// trailer/MDC) and the destination file's Close() errors are captured and
// joined rather than dropped via a bare defer: a flush failure here would
// otherwise produce a truncated, silently corrupt encrypted backup that
// still reports success - the same class of bug flagged in review for
// pkg/backup/dumper/archive.go's tar/gzip writers.
func streamEncrypt(srcPath string, dstPath string, open func(dst *os.File) (io.WriteCloser, error)) (err error) {
	src, err := os.Open(srcPath)
	if err != nil {
		return err
	}
	defer src.Close()

	dst, err := os.OpenFile(dstPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	defer func() {
		err = errors.Join(err, dst.Close())
	}()

	encWriter, err := open(dst)
	if err != nil {
		return err
	}

	if _, copyErr := io.Copy(encWriter, src); copyErr != nil {
		_ = encWriter.Close()
		return copyErr
	}
	return encWriter.Close()
}

// rsaEncryptor implements public-key (asymmetric) encryption, mirroring
// cloud-pgdumper's gpg_rsa_service.py.
type rsaEncryptor struct {
	publicKeyArmored string
}

func (e *rsaEncryptor) Encrypt(_ context.Context, srcPath string, dstPath string) error {
	key, err := crypto.NewKeyFromArmored(e.publicKeyArmored)
	if err != nil {
		return fmt.Errorf("invalid gpg-rsa public key: %w", err)
	}
	keyRing, err := crypto.NewKeyRing(key)
	if err != nil {
		return fmt.Errorf("unable to build gpg-rsa key ring: %w", err)
	}

	return streamEncrypt(srcPath, dstPath, func(dst *os.File) (io.WriteCloser, error) {
		return keyRing.EncryptStream(dst, nil, nil)
	})
}

// aesEncryptor implements passphrase-based symmetric encryption, mirroring
// cloud-pgdumper's gpg_aes_service.py. It calls go-crypto's
// openpgp.SymmetricallyEncrypt directly rather than gopenpgp's
// EncryptMessageWithPassword wrapper: the wrapper is genuinely streaming
// underneath, but buffers its own output into an in-memory bytes.Buffer
// before returning it, which defeats the point here.
type aesEncryptor struct {
	passphrase string
}

func (e *aesEncryptor) Encrypt(_ context.Context, srcPath string, dstPath string) error {
	return streamEncrypt(srcPath, dstPath, func(dst *os.File) (io.WriteCloser, error) {
		return openpgp.SymmetricallyEncrypt(dst, []byte(e.passphrase), nil, nil)
	})
}
