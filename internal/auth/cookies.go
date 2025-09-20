package auth

import (
    "net/http"
    "time"
)

type CookieOpts struct {
    Name     string
    Value    string
    Domain   string
    Secure   bool
    MaxAge   int           // seconds
    Path     string
    HTTPOnly bool
    // SameSite is fixed to None for cross-site in this scaffold
}

func SetCrossSiteCookie(w http.ResponseWriter, o CookieOpts) {
    c := &http.Cookie{
        Name:     o.Name,
        Value:    o.Value,
        Domain:   o.Domain,
        Path:     "/",
        HttpOnly: o.HTTPOnly,
        Secure:   o.Secure,
        SameSite: http.SameSiteNoneMode,
    }
    if o.Path != "" { c.Path = o.Path }
    if o.MaxAge > 0 {
        c.MaxAge = o.MaxAge
        c.Expires = time.Now().Add(time.Duration(o.MaxAge) * time.Second)
    }
    http.SetCookie(w, c)
}

func ClearCookie(w http.ResponseWriter, name, domain string, secure bool) {
    c := &http.Cookie{
        Name:     name,
        Value:    "",
        Domain:   domain,
        Path:     "/",
        MaxAge:   -1,
        Expires:  time.Unix(0,0),
        HttpOnly: true,
        Secure:   secure,
        SameSite: http.SameSiteNoneMode,
    }
    http.SetCookie(w, c)
}
