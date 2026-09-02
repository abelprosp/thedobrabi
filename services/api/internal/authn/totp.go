package authn

import (
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base32"
	"encoding/binary"
	"fmt"
	"strings"
	"time"
	"github.com/thedobra/thedobra/services/api/internal/cryptoenc"
)

func NewTOTPSecret() (string, error) {
	raw, err := cryptoenc.RandomBytes(20)
	if err != nil {
		return "", err
	}
	return base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(raw), nil
}

func TOTPNow(secret string) string {
	return totpAt(secret, time.Now().Unix()/30)
}

func VerifyTOTP(secret, code string) bool {
	code = strings.TrimSpace(code)
	if len(code) != 6 {
		return false
	}
	t := time.Now().Unix() / 30
	for _, d := range []int64{-1, 0, 1} {
		if totpAt(secret, t+d) == code {
			return true
		}
	}
	return false
}

func totpAt(secret string, counter int64) string {
	sec := strings.ToUpper(strings.ReplaceAll(secret, " ", ""))
	key, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(sec)
	if err != nil {
		return ""
	}
	var buf [8]byte
	binary.BigEndian.PutUint64(buf[:], uint64(counter))
	mac := hmac.New(sha1.New, key)
	mac.Write(buf[:])
	sum := mac.Sum(nil)
	off := sum[len(sum)-1] & 0x0f
	bin := (int(sum[off])&0x7f)<<24 | int(sum[off+1])<<16 | int(sum[off+2])<<8 | int(sum[off+3])
	return fmt.Sprintf("%06d", bin%1000000)
}

func OTPAuthURL(email, secret string) string {
	return fmt.Sprintf("otpauth://totp/TheDobra:%s?secret=%s&issuer=TheDobra&digits=6&period=30", email, secret)
}
