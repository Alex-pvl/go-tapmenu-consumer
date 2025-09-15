package tapmenu

import (
	"context"
	"errors"
	"fmt"
	"github.com/alex-pvl/go-tapmenu-consumer/internal/app/utils"
	"github.com/dgrijalva/jwt-go"
	"github.com/google/uuid"
	"net/http"
	"strings"
	"time"
)

func (s *Server) handleLogin() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		username := r.FormValue("username")
		password := r.FormValue("password")

		waiter, err := s.db.GetWaiterByUsername(username)

		if err != nil {
			s.logger.Error(err)
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}

		if !utils.CheckPasswordHash(password, waiter.HashedPassword) {
			s.logger.Error(err)
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		token, err := s.createToken(waiter.Id, waiter.Username)
		if err != nil {
			s.logger.Error(err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		renderJSON(w, map[string]string{
			"token": token,
		})
	}
}

func (s *Server) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authorization := r.Header.Get("Authorization")

		if authorization == "" {
			s.logger.Error("empty authorization header")
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		authorization = strings.Replace(authorization, "Bearer ", "", 1)

		token, err := jwt.Parse(authorization, func(token *jwt.Token) (interface{}, error) {
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
			}
			return []byte(s.configuration.JWTSecret), nil
		})

		if err != nil {
			s.logger.Error(err)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		if !token.Valid {
			s.logger.Error("token is invalid")
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok {
			s.logger.Error("cannot assert claims")
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		waiterId, ok := claims["waiter_id"].(string)
		if !ok {
			s.logger.Error("cannot assert waiter_id")
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		ctx := context.WithValue(r.Context(), "waiter_id", waiterId)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (s *Server) createToken(waiterId uuid.UUID, username string) (string, error) {
	claims := jwt.MapClaims{}
	claims["waiter_id"] = waiterId.String()
	claims["username"] = username
	claims["exp"] = time.Now().Add(time.Hour * 24).Unix()
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(s.configuration.JWTSecret))
}

func (s *Server) getWaiterIdFromContext(ctx context.Context) (string, error) {
	userID, ok := ctx.Value("waiter_id").(string)
	if !ok {
		return "", errors.New("cannot find waiter_id in context")
	}
	return userID, nil
}
