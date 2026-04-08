package middleware

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt/v5"
	appcontext "github.com/lucaspose/goci/internal/api/context"
	"github.com/lucaspose/goci/internal/api/response"
	"github.com/lucaspose/goci/internal/auth"
)

func RequireAuth(auth *auth.Service) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				response.WriteJSONError(w, http.StatusUnauthorized, "unauthorized")
				return
			}
			parts := strings.Split(authHeader, " ")
			if len(parts) != 2 || parts[0] != "Bearer" {
				response.WriteJSONError(w, http.StatusUnauthorized, "unauthorized")
				return
			}
			tokenString := parts[1]
			token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
				if token.Method.Alg() != "HS256" {
					return nil, errors.New("parsing secret error")
				}
				return []byte(auth.GetSecret()), nil
			})
			if err != nil {
				response.WriteJSONError(w, http.StatusUnauthorized, "unauthorized")
				return
			}
			if !token.Valid {
				response.WriteJSONError(w, http.StatusUnauthorized, "unauthorized")
				return
			}
			claims, ok := token.Claims.(jwt.MapClaims)
			if !ok {
				response.WriteJSONError(w, http.StatusUnauthorized, "unauthorized")
				return
			}
			userID, ok := claims["user_id"].(string)
			if !ok || userID == "" {
				response.WriteJSONError(w, http.StatusUnauthorized, "unauthorized")
				return
			}
			ctx := context.WithValue(r.Context(), appcontext.UserIDKey, userID)
			r = r.WithContext(ctx)
			next.ServeHTTP(w, r)
		})
	}
}
