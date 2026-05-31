package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

type Claims struct {
	Sub  string `json:"sub"`
	Role string `json:"role"`
	Exp  int64  `json:"exp"`
}

func Sign(secret string, claims Claims) (string, error) {
	header := map[string]string{"alg": "HS256", "typ": "JWT"}
	headerJSON, err := json.Marshal(header)
	if err != nil {
		return "", err
	}
	claimsJSON, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}
	unsigned := encode(headerJSON) + "." + encode(claimsJSON)
	return unsigned + "." + signature(secret, unsigned), nil
}

func Verify(secret, token string) (Claims, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return Claims{}, errors.New("invalid token")
	}
	unsigned := parts[0] + "." + parts[1]
	if !hmac.Equal([]byte(signature(secret, unsigned)), []byte(parts[2])) {
		return Claims{}, errors.New("invalid token signature")
	}
	claimsJSON, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return Claims{}, err
	}
	var claims Claims
	if err := json.Unmarshal(claimsJSON, &claims); err != nil {
		return Claims{}, err
	}
	if claims.Exp < time.Now().Unix() {
		return Claims{}, errors.New("expired token")
	}
	return claims, nil
}

func encode(data []byte) string {
	return base64.RawURLEncoding.EncodeToString(data)
}

func signature(secret, unsigned string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(unsigned))
	return encode(mac.Sum(nil))
}
