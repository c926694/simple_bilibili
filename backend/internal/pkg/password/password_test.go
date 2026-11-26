package password

import (
	"errors"
	"strings"
	"testing"

	"golang.org/x/crypto/bcrypt"
)

func TestHashAndCompare(t *testing.T) {
	for _, value := range []string{"bili-password", strings.Repeat("密", 64)} {
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
