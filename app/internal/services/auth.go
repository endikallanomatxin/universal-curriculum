package services

import (
	"context"
	"crypto/subtle"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"golang.org/x/crypto/bcrypt"
	"universal-curriculum/internal/db"
	"universal-curriculum/internal/models"
)

var ErrInvalidCredentials = errors.New("invalid credentials")

// A valid hash keeps unknown-user and wrong-password attempts on the same
// deliberately expensive comparison path.
var dummyPasswordHash = func() []byte {
	hash, err := bcrypt.GenerateFromPassword([]byte("not-a-real-password"), bcrypt.DefaultCost)
	if err != nil {
		panic(err)
	}
	return hash
}()

func EnsureBootstrapAdmin(database *sql.DB, fullName, alias, email, password string) error {
	if email == "" && password == "" {
		return nil
	}
	if strings.TrimSpace(fullName) == "" {
		return errors.New("bootstrap administrator full name is required")
	}
	if err := models.ValidatePassword(password); err != nil {
		return err
	}
	passwordHash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	_, err = db.CreateBootstrapAdmin(database, fullName, alias, email, passwordHash)
	return err
}

func RegisterLocalUser(database *sql.DB, fullName, email, password string) (*models.User, error) {
	fullName = strings.TrimSpace(fullName)
	email = models.NormalizeEmail(email)
	if err := models.ValidateFullName(fullName); err != nil {
		return nil, err
	}
	if err := models.ValidateEmail(email); err != nil {
		return nil, err
	}
	if err := models.ValidatePassword(password); err != nil {
		return nil, err
	}
	passwordHash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("hash local user password: %w", err)
	}
	return db.CreateLocalUser(database, fullName, email, passwordHash)
}

func AuthenticateLocal(database *sql.DB, email, password string) (*models.User, error) {
	userID, passwordHash, err := db.GetLocalPasswordHash(database, email)
	if err != nil {
		return nil, err
	}
	if passwordHash == nil {
		passwordHash = dummyPasswordHash
	}
	valid := bcrypt.CompareHashAndPassword(passwordHash, []byte(password)) == nil
	if userID == 0 || !valid {
		return nil, ErrInvalidCredentials
	}
	user, err := db.GetUserByID(database, userID)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, ErrInvalidCredentials
	}
	return user, nil
}

type sessionContext struct {
	UserID    int64
	CSRFToken string
}

type sessionContextKey struct{}

func WithSession(request *http.Request, userID int64, csrfToken string) *http.Request {
	ctx := context.WithValue(request.Context(), sessionContextKey{}, sessionContext{
		UserID: userID, CSRFToken: csrfToken,
	})
	return request.WithContext(ctx)
}

func SessionUserID(request *http.Request) (int64, bool) {
	session, ok := request.Context().Value(sessionContextKey{}).(sessionContext)
	return session.UserID, ok && session.UserID != 0
}

func SessionCSRFToken(request *http.Request) (string, bool) {
	session, ok := request.Context().Value(sessionContextKey{}).(sessionContext)
	return session.CSRFToken, ok && session.CSRFToken != ""
}

func ValidCSRFToken(request *http.Request, submitted string) bool {
	expected, ok := SessionCSRFToken(request)
	if !ok || len(expected) != len(submitted) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(expected), []byte(submitted)) == 1
}
