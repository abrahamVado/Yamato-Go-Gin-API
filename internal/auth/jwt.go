package auth

import (
    "errors"
    "time"
    "github.com/golang-jwt/jwt/v5"
)

func SignHS256(subject, secret string, ttl time.Duration, extra map[string]any) (string, error) {
    if secret == "" {
        return "", errors.New("missing JWT secret")
    }
    claims := jwt.MapClaims{
        "sub": subject,
        "iat": time.Now().Unix(),
        "exp": time.Now().Add(ttl).Unix(),
    }
    for k, v := range extra {
        claims[k] = v
    }
    token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
    return token.SignedString([]byte(secret))
}

func ParseHS256(tokenStr, secret string) (*jwt.Token, jwt.MapClaims, error) {
    token, err := jwt.Parse(tokenStr, func(token *jwt.Token) (interface{}, error) {
        if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
            return nil, errors.New("unexpected signing method")
        }
        return []byte(secret), nil
    })
    if err != nil {
        return nil, nil, err
    }
    if !token.Valid {
        return nil, nil, errors.New("invalid token")
    }
    if claims, ok := token.Claims.(jwt.MapClaims); ok {
        return token, claims, nil
    }
    return token, nil, errors.New("invalid claims")
}
