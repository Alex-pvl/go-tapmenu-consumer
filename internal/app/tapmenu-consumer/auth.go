package tapmenu

import (
	"errors"
	"github.com/alex-pvl/go-tapmenu-consumer/internal/app/utils"
	"net/http"
	"time"
)

var AuthError = errors.New("unauthorized")

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

		sessionToken := utils.GenerateToken(32)
		csrfToken := utils.GenerateToken(32)

		http.SetCookie(w, &http.Cookie{
			Name:     "session_token",
			Value:    sessionToken,
			Expires:  time.Now().Add(24 * time.Hour),
			HttpOnly: true,
		})
		http.SetCookie(w, &http.Cookie{
			Name:     "csrf_token",
			Value:    csrfToken,
			Expires:  time.Now().Add(24 * time.Hour),
			HttpOnly: false,
		})

		waiter.SessionToken = sessionToken
		waiter.CSRFToken = csrfToken

		err = s.db.UpdateWaiter(waiter)
		if err != nil {
			s.logger.Error(err)
		}

		s.logger.Infof("waiter [%s] logged in", username)
	}
}

func (s *Server) authorize(r *http.Request) error {
	session, err := r.Cookie("session_token")
	if err != nil {
		return AuthError
	}

	waiter, err := s.db.GetWaiterBySession(session.Value)
	if err != nil || session.Value != waiter.SessionToken {
		s.logger.Error("session token mismatch")
		return AuthError
	}

	csrf := r.Header.Get("X-CSRF-Token")
	if csrf == "" || csrf != waiter.CSRFToken {
		s.logger.Error("csrf token mismatch")
		return AuthError
	}

	return nil
}

func (s *Server) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := s.authorize(r); err != nil {
			s.logger.Error("unauthorized: ", err)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}
