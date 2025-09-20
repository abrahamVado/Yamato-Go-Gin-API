package config

import (
    "errors"
    "os"
    "strings"
    "time"
)

type Config struct {
    Port           string
    DatabaseURL    string
    CORSOrigins    []string

    CookieDomain   string
    CookieSecure   bool
    CookieRefresh  string
    CookieAccess   string
    AccessTTL      time.Duration
    RefreshTTL     time.Duration

    JWTAlg         string
    JWTSecret      string

    SMTPURL        string
    MailFromAddr   string
    MailFromName   string
}

func getEnv(key, def string) string {
    if v := os.Getenv(key); v != "" {
        return v
    }
    return def
}

func Load() (*Config, error) {
    c := &Config{
        Port:          getEnv("PORT", "8083"),
        DatabaseURL:   getEnv("DATABASE_URL", ""),
        CookieDomain:  getEnv("COOKIE_DOMAIN", ""),
        CookieRefresh: getEnv("COOKIE_REFRESH", "yamato_rt"),
        CookieAccess:  getEnv("COOKIE_ACCESS", "yamato_at"),
        JWTAlg:        getEnv("JWT_ALG", "HS256"),
        JWTSecret:     getEnv("JWT_SECRET", ""),
        SMTPURL:       getEnv("SMTP_URL", ""),
        MailFromAddr:  getEnv("MAIL_FROM_ADDRESS", ""),
        MailFromName:  getEnv("MAIL_FROM_NAME", "Yamato Auth"),
    }

    c.CookieSecure = strings.ToLower(getEnv("COOKIE_SECURE", "true")) == "true"

    if dur, err := time.ParseDuration(getEnv("ACCESS_TTL", "10m")); err == nil {
        c.AccessTTL = dur
    } else {
        return nil, err
    }
    if dur, err := time.ParseDuration(getEnv("REFRESH_TTL", "720h")); err == nil {
        c.RefreshTTL = dur
    } else {
        return nil, err
    }

    origins := strings.Split(getEnv("CORS_ORIGINS", ""), ",")
    c.CORSOrigins = nil
    for _, o := range origins {
        o = strings.TrimSpace(o)
        if o != "" { c.CORSOrigins = append(c.CORSOrigins, o) }
    }

    if c.JWTAlg != "HS256" {
        return nil, errors.New("only HS256 supported in this scaffold; set JWT_ALG=HS256")
    }
    if c.JWTSecret == "" {
        // Allow empty for now, but warn; handlers that need JWT should check and return 500
    }
    return c, nil
}
