package password

import (
	"crypto/sha256"
	"encoding/base64"
	"strings"

	"golang.org/x/crypto/bcrypt"
)

const (
	bcryptLimit = 72
	longPrefix  = "sha256:"
)

func Hash(value string) (string, error) {
	if len(value) > bcryptLimit {
		hash, err := bcrypt.GenerateFromPassword(digest(value), bcrypt.DefaultCost)
		if err != nil {
			return "", err
		}
		return longPrefix + string(hash), nil
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(value), bcrypt.DefaultCost)
	return string(hash), err
}

func Compare(hash, value string) error {
	if stored, ok := strings.CutPrefix(hash, longPrefix); ok {
		return bcrypt.CompareHashAndPassword([]byte(stored), digest(value))
	}
	if len(value) > bcryptLimit {
		return bcrypt.ErrMismatchedHashAndPassword
	}
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(value))
}

func digest(value string) []byte {
	// bcrypt 的输入最多为 72 字节，先将长密码编码为固定长度的摘要。
	sum := sha256.Sum256([]byte(value))
	return []byte(base64.RawStdEncoding.EncodeToString(sum[:]))
}
