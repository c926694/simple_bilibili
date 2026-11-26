package password

import (
	"errors"
	"strings"
	"testing"

	"golang.org/x/crypto/bcrypt"
)

func TestHashAndCompare(t *testing.T) {
	for _, value := range []string{"1", "\x00 \n", strings.Repeat("密", 1024)} {
		first, err := Hash(value)
		if err != nil {
			t.Fatal(err)
		}
		second, err := Hash(value)
		if err != nil {
			t.Fatal(err)
		}
		if first == second {
			t.Fatal("相同密码的两次哈希应使用不同的盐值")
		}
		if err := Compare(first, value); err != nil {
			t.Fatal(err)
		}
		if err := Compare(first, value+"x"); !errors.Is(err, bcrypt.ErrMismatchedHashAndPassword) {
			t.Fatalf("密码不匹配时返回 %v", err)
		}
	}
}

func TestCompareLegacyBcrypt(t *testing.T) {
	const value = "existing-password"
	hash, err := bcrypt.GenerateFromPassword([]byte(value), bcrypt.DefaultCost)
	if err != nil {
		t.Fatal(err)
	}
	if err := Compare(string(hash), value); err != nil {
		t.Fatal(err)
	}
}
