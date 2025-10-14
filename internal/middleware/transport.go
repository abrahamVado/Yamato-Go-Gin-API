package middleware

import (
	"github.com/gin-contrib/gzip"
	"github.com/gin-gonic/gin"
)

// Gzip compresses responses using gzip with a sensible default compression level.
func Gzip() gin.HandlerFunc {
	// 1.- Leverage gin-contrib/gzip to create the middleware with default compression settings.
	return gzip.Gzip(gzip.DefaultCompression)
}

// Recovery defends against panics in handlers and converts them into 500 responses.
func Recovery() gin.HandlerFunc {
	// 1.- Delegate to Gin's built-in recovery middleware for battle-tested behaviour.
	return gin.Recovery()
}
