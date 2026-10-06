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
package encryption

import (
	"context"
	"fmt"
	"os"

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

func readWriteEncrypted(srcPath string, dstPath string, encrypt func(plaintext []byte) ([]byte, error)) error {
	plaintext, err := os.ReadFile(srcPath)
	if err != nil {
		return err
	}
	ciphertext, err := encrypt(plaintext)
	if err != nil {
		return err
	}
	return os.WriteFile(dstPath, ciphertext, 0o600)
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

	return readWriteEncrypted(srcPath, dstPath, func(plaintext []byte) ([]byte, error) {
		message := crypto.NewPlainMessage(plaintext)
		encrypted, err := keyRing.Encrypt(message, nil)
		if err != nil {
			return nil, err
		}
		return encrypted.GetBinary(), nil
	})
}

// aesEncryptor implements passphrase-based symmetric encryption, mirroring
// cloud-pgdumper's gpg_aes_service.py.
type aesEncryptor struct {
	passphrase string
}

func (e *aesEncryptor) Encrypt(_ context.Context, srcPath string, dstPath string) error {
	return readWriteEncrypted(srcPath, dstPath, func(plaintext []byte) ([]byte, error) {
		message := crypto.NewPlainMessage(plaintext)
		encrypted, err := crypto.EncryptMessageWithPassword(message, []byte(e.passphrase))
		if err != nil {
			return nil, err
		}
		return encrypted.GetBinary(), nil
	})
}
