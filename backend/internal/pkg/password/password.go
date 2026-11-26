package password

import (
	"crypto/sha256"
	"encoding/base64"
	"strings"

	"golang.org/x/crypto/bcrypt"
)

const (
	bcryptLimit  = 72
	digestPrefix = "sha256:"
)

func Hash(value string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword(digest(value), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return digestPrefix + string(hash), nil
}

func Compare(hash, value string) error {
	if stored, ok := strings.CutPrefix(hash, digestPrefix); ok {
		return bcrypt.CompareHashAndPassword([]byte(stored), digest(value))
	}
	if len(value) > bcryptLimit {
		return bcrypt.ErrMismatchedHashAndPassword
	}
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(value))
}

func digest(value string) []byte {
	// 将任意长度的密码编码为固定长度的摘要，满足 bcrypt 的输入长度要求。
	sum := sha256.Sum256([]byte(value))
	return []byte(base64.RawStdEncoding.EncodeToString(sum[:]))
}
