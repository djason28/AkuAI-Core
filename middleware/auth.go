package middleware

import (
	"AkuAI/pkg/config"
	tokenstore "AkuAI/pkg/token"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

const (
	ContextUserIDKey = "current_user_id"
	ContextJTIKey    = "current_jti"
)

func AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		auth := c.GetHeader("Authorization")
		if auth == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"msg": "missing authorization header"})
			return
		}
		parts := strings.Fields(auth)
		if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"msg": "invalid authorization header"})
			return
		}
		tokenStr := parts[1]

		token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (interface{}, error) {
			// only accept HMAC signing
			if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, jwt.ErrTokenUnverifiable
			}
			return []byte(config.JWTSecret), nil
		})
		if err != nil || !token.Valid {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"msg": "invalid token"})
			return
		}

		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"msg": "invalid token claims"})
			return
		}

		// jti
		jtiVal, _ := claims["jti"].(string)
		if tokenstore.IsRevoked(jtiVal) {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"msg": "Token has been revoked (logout)"})
			return
		}

		// subject (user id)
		var userIDStr string
		if sub, ok := claims["sub"].(string); ok {
			userIDStr = sub
		} else if subf, ok := claims["sub"].(float64); ok {
			// jwt lib may parse numeric as float64
			userIDStr = strconv.Itoa(int(subf))
		}

		if userIDStr == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"msg": "invalid subject in token"})
			return
		}

		// set to context
		c.Set(ContextUserIDKey, userIDStr)
		c.Set(ContextJTIKey, jtiVal)
		c.Next()
	}
}
