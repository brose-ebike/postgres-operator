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

package encryption

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/ProtonMail/gopenpgp/v2/crypto"
	"github.com/ProtonMail/gopenpgp/v2/helper"

	apiV1 "github.com/brose-ebike/postgres-operator/api/v1"
)

const testPlaintext = "dummy pg_dump contents for encryption round-trip tests"

func writeTempFile(t *testing.T, contents string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "dump")
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatalf("unable to write temp file: %v", err)
	}
	return path
}

func TestRSAEncryptor_RoundTrip(t *testing.T) {
	armoredPrivate, err := helper.GenerateKey("test", "test@example.com", []byte("key-passphrase"), "rsa", 2048)
	if err != nil {
		t.Fatalf("unable to generate test rsa key: %v", err)
	}
	privateKey, err := crypto.NewKeyFromArmored(armoredPrivate)
	if err != nil {
		t.Fatalf("unable to parse generated private key: %v", err)
	}
	armoredPublic, err := privateKey.GetArmoredPublicKey()
	if err != nil {
		t.Fatalf("unable to extract public key: %v", err)
	}

	enc, err := NewEncryptor(apiV1.PgBackupEncryptionTypeGPGRSA, armoredPublic)
	if err != nil {
		t.Fatalf("NewEncryptor failed: %v", err)
	}

	srcPath := writeTempFile(t, testPlaintext)
	dstPath := filepath.Join(t.TempDir(), "dump.gpg")

	if err := enc.Encrypt(context.Background(), srcPath, dstPath); err != nil {
		t.Fatalf("Encrypt failed: %v", err)
	}

	ciphertext, err := os.ReadFile(dstPath)
	if err != nil {
		t.Fatalf("unable to read encrypted output: %v", err)
	}
	if bytes.Contains(ciphertext, []byte(testPlaintext)) {
		t.Fatalf("encrypted output must not contain the plaintext verbatim")
	}

	unlockedPrivate, err := privateKey.Unlock([]byte("key-passphrase"))
	if err != nil {
		t.Fatalf("unable to unlock private key: %v", err)
	}
	keyRing, err := crypto.NewKeyRing(unlockedPrivate)
	if err != nil {
		t.Fatalf("unable to build key ring: %v", err)
	}

	plain, err := keyRing.Decrypt(crypto.NewPGPMessage(ciphertext), nil, 0)
	if err != nil {
		t.Fatalf("Decrypt with the matching private key failed: %v", err)
	}
	if string(plain.GetBinary()) != testPlaintext {
		t.Fatalf("decrypted content does not match original, got %q", plain.GetBinary())
	}
}

func TestRSAEncryptor_WrongKeyFailsToDecrypt(t *testing.T) {
	armoredPrivateA, err := helper.GenerateKey("a", "a@example.com", []byte("pw"), "rsa", 2048)
	if err != nil {
		t.Fatalf("unable to generate key A: %v", err)
	}
	privateKeyA, err := crypto.NewKeyFromArmored(armoredPrivateA)
	if err != nil {
		t.Fatalf("unable to parse key A: %v", err)
	}
	armoredPublicA, err := privateKeyA.GetArmoredPublicKey()
	if err != nil {
		t.Fatalf("unable to extract public key A: %v", err)
	}

	armoredPrivateB, err := helper.GenerateKey("b", "b@example.com", []byte("pw"), "rsa", 2048)
	if err != nil {
		t.Fatalf("unable to generate key B: %v", err)
	}
	privateKeyB, err := crypto.NewKeyFromArmored(armoredPrivateB)
	if err != nil {
		t.Fatalf("unable to parse key B: %v", err)
	}

	enc, err := NewEncryptor(apiV1.PgBackupEncryptionTypeGPGRSA, armoredPublicA)
	if err != nil {
		t.Fatalf("NewEncryptor failed: %v", err)
	}
	srcPath := writeTempFile(t, testPlaintext)
	dstPath := filepath.Join(t.TempDir(), "dump.gpg")
	if err := enc.Encrypt(context.Background(), srcPath, dstPath); err != nil {
		t.Fatalf("Encrypt failed: %v", err)
	}
	ciphertext, err := os.ReadFile(dstPath)
	if err != nil {
		t.Fatalf("unable to read encrypted output: %v", err)
	}

	unlockedB, err := privateKeyB.Unlock([]byte("pw"))
	if err != nil {
		t.Fatalf("unable to unlock key B: %v", err)
	}
	keyRingB, err := crypto.NewKeyRing(unlockedB)
	if err != nil {
		t.Fatalf("unable to build key ring B: %v", err)
	}

	if _, err := keyRingB.Decrypt(crypto.NewPGPMessage(ciphertext), nil, 0); err == nil {
		t.Fatal("expected decryption with the wrong private key to fail")
	}
}

func TestAESEncryptor_RoundTrip(t *testing.T) {
	enc, err := NewEncryptor(apiV1.PgBackupEncryptionTypeGPGAES, "correct-horse-battery-staple")
	if err != nil {
		t.Fatalf("NewEncryptor failed: %v", err)
	}

	srcPath := writeTempFile(t, testPlaintext)
	dstPath := filepath.Join(t.TempDir(), "dump.gpg")

	if err := enc.Encrypt(context.Background(), srcPath, dstPath); err != nil {
		t.Fatalf("Encrypt failed: %v", err)
	}

	ciphertext, err := os.ReadFile(dstPath)
	if err != nil {
		t.Fatalf("unable to read encrypted output: %v", err)
	}
	if bytes.Contains(ciphertext, []byte(testPlaintext)) {
		t.Fatalf("encrypted output must not contain the plaintext verbatim")
	}

	plain, err := crypto.DecryptMessageWithPassword(crypto.NewPGPMessage(ciphertext), []byte("correct-horse-battery-staple"))
	if err != nil {
		t.Fatalf("Decrypt with the matching passphrase failed: %v", err)
	}
	if string(plain.GetBinary()) != testPlaintext {
		t.Fatalf("decrypted content does not match original, got %q", plain.GetBinary())
	}
}

func TestAESEncryptor_WrongPassphraseFailsToDecrypt(t *testing.T) {
	enc, err := NewEncryptor(apiV1.PgBackupEncryptionTypeGPGAES, "correct-horse-battery-staple")
	if err != nil {
		t.Fatalf("NewEncryptor failed: %v", err)
	}
	srcPath := writeTempFile(t, testPlaintext)
	dstPath := filepath.Join(t.TempDir(), "dump.gpg")
	if err := enc.Encrypt(context.Background(), srcPath, dstPath); err != nil {
		t.Fatalf("Encrypt failed: %v", err)
	}
	ciphertext, err := os.ReadFile(dstPath)
	if err != nil {
		t.Fatalf("unable to read encrypted output: %v", err)
	}

	if _, err := crypto.DecryptMessageWithPassword(crypto.NewPGPMessage(ciphertext), []byte("wrong-passphrase")); err == nil {
		t.Fatal("expected decryption with the wrong passphrase to fail")
	}
}
