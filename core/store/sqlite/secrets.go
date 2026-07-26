package sqlite

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"danmo-work/core/port"

	"gorm.io/gorm"
)

type secretModel struct {
	Key       string `gorm:"primaryKey"`
	Value     string `gorm:"column:value"` // base64(nonce|ciphertext)
	UpdatedAt int64  `gorm:"column:updated_at"`
}

func (secretModel) TableName() string { return "secrets" }

type secretStore struct {
	db  *gorm.DB
	key []byte
}

func newSecretStore(db *gorm.DB) *secretStore {
	return &secretStore{db: db, key: deriveSecretKey()}
}

func deriveSecretKey() []byte {
	seed := os.Getenv("WORK_SECRET_KEY")
	if seed == "" {
		// Machine-local fallback: hostname + home path. Not perfect HSM, but
		// keeps tokens out of plaintext YAML and model prompts.
		home, _ := os.UserHomeDir()
		host, _ := os.Hostname()
		seed = "danmo-work|" + host + "|" + home
	}
	sum := sha256.Sum256([]byte(seed))
	return sum[:]
}

func (s *secretStore) Put(ctx context.Context, key, value string) error {
	enc, err := s.encrypt(value)
	if err != nil {
		return err
	}
	row := secretModel{Key: key, Value: enc, UpdatedAt: time.Now().Unix()}
	return s.db.WithContext(ctx).Save(&row).Error
}

func (s *secretStore) Get(ctx context.Context, key string) (string, error) {
	var row secretModel
	if err := s.db.WithContext(ctx).Where("key = ?", key).First(&row).Error; err != nil {
		return "", err
	}
	return s.decrypt(row.Value)
}

func (s *secretStore) Delete(ctx context.Context, key string) error {
	return s.db.WithContext(ctx).Where("key = ?", key).Delete(&secretModel{}).Error
}

func (s *secretStore) DeletePrefix(ctx context.Context, prefix string) error {
	return s.db.WithContext(ctx).Where("key LIKE ?", prefix+"%").Delete(&secretModel{}).Error
}

func (s *secretStore) encrypt(plain string) (string, error) {
	block, err := aes.NewCipher(s.key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	out := gcm.Seal(nonce, nonce, []byte(plain), nil)
	return base64.StdEncoding.EncodeToString(out), nil
}

func (s *secretStore) decrypt(encoded string) (string, error) {
	raw, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", err
	}
	block, err := aes.NewCipher(s.key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	if len(raw) < gcm.NonceSize() {
		return "", fmt.Errorf("ciphertext too short")
	}
	nonce, ct := raw[:gcm.NonceSize()], raw[gcm.NonceSize():]
	plain, err := gcm.Open(nil, nonce, ct, nil)
	if err != nil {
		return "", err
	}
	return string(plain), nil
}

var _ port.SecretStore = (*secretStore)(nil)

// ListSecretKeys is used by tests / admin; values are never returned.
func ListSecretKeys(db *gorm.DB, prefix string) ([]string, error) {
	var rows []secretModel
	q := db.Model(&secretModel{})
	if prefix != "" {
		q = q.Where("key LIKE ?", prefix+"%")
	}
	if err := q.Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]string, len(rows))
	for i, r := range rows {
		out[i] = r.Key
	}
	return out, nil
}

func secretKeyHasPrefix(key, prefix string) bool {
	return strings.HasPrefix(key, prefix)
}
