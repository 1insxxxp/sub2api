package handler

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

const (
	checkinBrowserCookie   = "sub2_browser_id"
	browserLifetime        = 90 * 24 * time.Hour
	browserSignerContext   = "sub2api.browserCookieKey"
	browserIdentityContext = "sub2api.browserIdentity"
)

type browserIdentity struct {
	hash    string
	present bool
}

// Standalone handlers without a server configuration get a process-local signing key.
var standaloneBrowserCookieKey = sync.OnceValue(func() []byte { return []byte(rand.Text()) })

// BrowserCookieSigner supplies a domain-separated key shared by the panel instances.
func BrowserCookieSigner(secret string) gin.HandlerFunc {
	signer := hmac.New(sha256.New, []byte(secret))
	_, _ = signer.Write([]byte("sub2api.browser-cookie.v1"))
	key := signer.Sum(nil)
	return func(c *gin.Context) { c.Set(browserSignerContext, key); c.Next() }
}

func checkinBrowserIdentity(c *gin.Context) (string, bool, error) {
	if cached, ok := c.Get(browserIdentityContext); ok {
		v, ok := cached.(browserIdentity)
		if !ok {
			return "", false, errors.New("invalid cached browser identity")
		}
		return v.hash, v.present, nil
	}
	key, ok := c.Get(browserSignerContext)
	if !ok {
		key = standaloneBrowserCookieKey()
	}
	signingKey, ok := key.([]byte)
	if !ok {
		return "", false, errors.New("invalid browser signing key")
	}
	mac := hmac.New(sha256.New, signingKey)
	value, _ := c.Cookie(checkinBrowserCookie)
	payload, _ := base64.RawURLEncoding.DecodeString(value)
	const dataSize = 8 + 32
	now := time.Now()
	valid := false
	if len(payload) == dataSize+sha256.Size {
		_, _ = mac.Write(payload[:dataSize])
		expires := int64(binary.BigEndian.Uint64(payload[:8]))
		valid = expires > now.Unix() && hmac.Equal(mac.Sum(nil), payload[dataSize:])
	}
	if !valid {
		payload = make([]byte, dataSize)
		binary.BigEndian.PutUint64(payload[:8], uint64(now.Add(browserLifetime).Unix()))
		if _, err := rand.Read(payload[8:]); err != nil {
			return "", false, err
		}
		mac.Reset()
		_, _ = mac.Write(payload)
		payload = mac.Sum(payload)
		http.SetCookie(c.Writer, &http.Cookie{Name: checkinBrowserCookie, Value: base64.RawURLEncoding.EncodeToString(payload), Path: "/", MaxAge: int(browserLifetime.Seconds()), Secure: true, HttpOnly: true, SameSite: http.SameSiteLaxMode})
	}
	digest := sha256.Sum256(payload[8:dataSize])
	hash := hex.EncodeToString(digest[:])
	c.Set(browserIdentityContext, browserIdentity{hash, valid})
	return hash, valid, nil
}
