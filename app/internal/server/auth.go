package server

import (
	"errors"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"universal-curriculum/internal/db"
	"universal-curriculum/internal/models"
	"universal-curriculum/internal/services"
)

const invalidLoginMessage = "Incorrect email or password."

type authPageData struct {
	Error  string
	Notice string
	Next   string
}

func (server *Server) contributorInvitation(writer http.ResponseWriter, request *http.Request) {
	token := request.URL.Query().Get("token")
	if request.Method == http.MethodPost {
		request.Body = http.MaxBytesReader(writer, request.Body, 1<<20)
		if err := request.ParseForm(); err != nil {
			http.Error(writer, "Invalid invitation", http.StatusBadRequest)
			return
		}
		token = request.FormValue("token")
		if !services.ValidCSRFToken(request, request.FormValue("csrf_token")) {
			http.Error(writer, "Invalid CSRF token", http.StatusForbidden)
			return
		}
		userID, ok := services.SessionUserID(request)
		if !ok {
			http.Redirect(writer, request, "/auth/login?next="+url.QueryEscape("/auth/contributor-invitation?token="+token), http.StatusSeeOther)
			return
		}
		if err := db.AcceptContributorInvitation(server.Database, token, userID); err != nil {
			server.renderStatus(writer, http.StatusBadRequest, "contributor-invitation.html", contributorInvitationPageData{Token: token, Error: "This invitation is invalid, expired or belongs to another email address."})
			return
		}
		http.Redirect(writer, request, "/curriculum-modification", http.StatusSeeOther)
		return
	}
	data := contributorInvitationPageData{Token: token}
	if userID, ok := services.SessionUserID(request); ok {
		data.User, _ = db.GetUserByID(server.Database, userID)
		data.CSRFToken, _ = services.SessionCSRFToken(request)
	}
	server.render(writer, "contributor-invitation.html", data)
}

type registrationPageData struct {
	Error           string
	Next            string
	InvitationToken string
}

type contributorInvitationPageData struct {
	Token     string
	Error     string
	User      *models.User
	CSRFToken string
}

type forgotPasswordPageData struct {
	Error     string
	Requested bool
}

type resetPasswordPageData struct {
	Error   string
	Token   string
	Invalid bool
}

func (server *Server) login(writer http.ResponseWriter, request *http.Request) {
	if _, authenticated := services.SessionUserID(request); authenticated && request.Method == http.MethodGet {
		http.Redirect(writer, request, "/account", http.StatusSeeOther)
		return
	}

	switch request.Method {
	case http.MethodGet:
		data := authPageData{Next: safeRedirectPath(request.URL.Query().Get("next"), "/account")}
		if request.URL.Query().Get("registered") == "1" {
			data.Notice = "Account created. Log in to continue."
		}
		if request.URL.Query().Get("password-reset") == "1" {
			data.Notice = "Your password has been updated. Log in with your new password."
		}
		server.render(writer, "login.html", data)
	case http.MethodPost:
		request.Body = http.MaxBytesReader(writer, request.Body, 1<<20)
		if err := request.ParseForm(); err != nil {
			server.renderStatus(writer, http.StatusBadRequest, "login.html", authPageData{Error: invalidLoginMessage})
			return
		}
		email := models.NormalizeEmail(request.FormValue("email"))
		password := request.FormValue("password")
		next := safeRedirectPath(request.FormValue("next"), "/account")
		blocked, err := server.registerAuthenticationRateEvent(
			request,
			(*services.AuthenticationRateLimiter).RegisterLogin,
		)
		if err != nil {
			log.Printf("rate limit login: %v", err)
			http.Error(writer, "Unable to log in", http.StatusInternalServerError)
			return
		}
		if blocked {
			writer.Header().Set("Retry-After", "900")
			server.renderStatus(writer, http.StatusTooManyRequests, "login.html", authPageData{
				Error: "Too many attempts. Try again later.",
				Next:  next,
			})
			return
		}
		if email == "" || password == "" {
			server.renderStatus(writer, http.StatusUnauthorized, "login.html", authPageData{Error: invalidLoginMessage, Next: next})
			return
		}

		user, err := services.AuthenticateLocal(server.Database, email, password)
		if errors.Is(err, services.ErrInvalidCredentials) {
			server.renderStatus(writer, http.StatusUnauthorized, "login.html", authPageData{Error: invalidLoginMessage, Next: next})
			return
		}
		if err != nil {
			log.Printf("authenticate local user: %v", err)
			http.Error(writer, "Unable to log in", http.StatusInternalServerError)
			return
		}
		token, err := db.CreateSession(server.Database, user.ID)
		if err != nil {
			log.Printf("create session: %v", err)
			http.Error(writer, "Unable to log in", http.StatusInternalServerError)
			return
		}
		setSessionCookie(writer, token, server.Config.IsProd(), time.Now())
		http.Redirect(writer, request, next, http.StatusSeeOther)
	default:
		writer.Header().Set("Allow", "GET, POST")
		http.Error(writer, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (server *Server) registerAuthenticationRateEvent(
	request *http.Request,
	register func(*services.AuthenticationRateLimiter, string) (bool, error),
) (bool, error) {
	clientIP, err := ClientIP(request)
	if err != nil {
		return false, err
	}
	return register(services.NewAuthenticationRateLimiter(server.Database), clientIP.String())
}

func (server *Server) register(writer http.ResponseWriter, request *http.Request) {
	if _, authenticated := services.SessionUserID(request); authenticated && request.Method == http.MethodGet {
		http.Redirect(writer, request, "/account", http.StatusSeeOther)
		return
	}

	switch request.Method {
	case http.MethodGet:
		server.render(writer, "register.html", registrationPageData{
			Next:            safeRedirectPath(request.URL.Query().Get("next"), "/account"),
			InvitationToken: request.URL.Query().Get("invitation"),
		})
	case http.MethodPost:
		request.Body = http.MaxBytesReader(writer, request.Body, 1<<20)
		if err := request.ParseForm(); err != nil {
			server.renderStatus(writer, http.StatusBadRequest, "register.html", registrationPageData{
				Error: "Unable to create an account with those details.",
				Next:  "/account",
			})
			return
		}

		fullName := strings.TrimSpace(request.FormValue("full_name"))
		email := models.NormalizeEmail(request.FormValue("email"))
		password := request.FormValue("password")
		next := safeRedirectPath(request.FormValue("next"), "/account")
		invitationToken := request.FormValue("invitation_token")
		if invitationToken != "" {
			invitedEmail, invitationErr := db.ValidContributorInvitationEmail(server.Database, invitationToken)
			if invitationErr != nil || invitedEmail != email {
				server.renderStatus(writer, http.StatusBadRequest, "register.html", registrationPageData{Error: "The contributor invitation is invalid, expired or belongs to another email address.", Next: next, InvitationToken: invitationToken})
				return
			}
		}
		blocked, err := server.registerAuthenticationRateEvent(
			request,
			(*services.AuthenticationRateLimiter).RegisterRegistration,
		)
		if err != nil {
			log.Printf("rate limit registration: %v", err)
			http.Error(writer, "Unable to create account", http.StatusInternalServerError)
			return
		}
		if blocked {
			writer.Header().Set("Retry-After", "900")
			server.renderStatus(writer, http.StatusTooManyRequests, "register.html", registrationPageData{
				Error: "Too many attempts. Try again later.",
				Next:  next,
			})
			return
		}
		user, err := services.RegisterLocalUser(server.Database, fullName, email, password)
		if err != nil {
			data := registrationPageData{Next: next, InvitationToken: invitationToken}
			switch {
			case errors.Is(err, models.ErrFullNameRequired):
				data.Error = "Enter your name."
			case errors.Is(err, models.ErrFullNameTooLong):
				data.Error = "Your name must be 200 characters or fewer."
			case errors.Is(err, models.ErrInvalidEmail):
				data.Error = "Enter a valid email address."
			case errors.Is(err, models.ErrPasswordTooShort):
				data.Error = "Use at least 10 characters for your password."
			case errors.Is(err, models.ErrPasswordTooLong):
				data.Error = "Your password must be 72 bytes or fewer."
			case errors.Is(err, db.ErrEmailAlreadyRegistered):
				data.Error = "Unable to create an account with those details."
			default:
				log.Printf("register local user: %v", err)
				http.Error(writer, "Unable to create account", http.StatusInternalServerError)
				return
			}
			server.renderStatus(writer, http.StatusBadRequest, "register.html", data)
			return
		}
		if invitationToken != "" {
			if err := db.AcceptContributorInvitation(server.Database, invitationToken, user.ID); err != nil {
				server.renderStatus(writer, http.StatusBadRequest, "register.html", registrationPageData{Error: "The contributor invitation is invalid or expired.", Next: next, InvitationToken: invitationToken})
				return
			}
		}

		location := "/auth/login?registered=1&next=" + url.QueryEscape(next)
		http.Redirect(writer, request, location, http.StatusSeeOther)
	default:
		writer.Header().Set("Allow", "GET, POST")
		http.Error(writer, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (server *Server) forgotPassword(writer http.ResponseWriter, request *http.Request) {
	switch request.Method {
	case http.MethodGet:
		server.render(writer, "forgot-password.html", forgotPasswordPageData{
			Requested: request.URL.Query().Get("requested") == "1",
		})
	case http.MethodPost:
		request.Body = http.MaxBytesReader(writer, request.Body, 1<<20)
		if err := request.ParseForm(); err != nil {
			server.renderStatus(writer, http.StatusBadRequest, "forgot-password.html", forgotPasswordPageData{
				Error: "Unable to process that request.",
			})
			return
		}
		email := models.NormalizeEmail(request.FormValue("email"))
		if err := models.ValidateEmail(email); err != nil {
			server.renderStatus(writer, http.StatusBadRequest, "forgot-password.html", forgotPasswordPageData{
				Error: "Enter a valid email address.",
			})
			return
		}
		clientIP, err := ClientIP(request)
		if err != nil {
			log.Printf("resolve password reset request IP: %v", err)
			server.renderStatus(writer, http.StatusBadRequest, "forgot-password.html", forgotPasswordPageData{
				Error: "Unable to process that request.",
			})
			return
		}
		blocked, err := services.NewAuthenticationRateLimiter(server.Database).RegisterPasswordResetRequest(clientIP.String())
		if err != nil {
			log.Printf("rate limit password reset request: %v", err)
			http.Error(writer, "Unable to request password reset", http.StatusInternalServerError)
			return
		}
		if blocked {
			writer.Header().Set("Retry-After", "900")
			server.renderStatus(writer, http.StatusTooManyRequests, "forgot-password.html", forgotPasswordPageData{
				Error: "Too many requests. Try again later.",
			})
			return
		}
		if err := services.RequestPasswordReset(
			request.Context(),
			server.Database,
			server.EmailSender,
			server.Config.AppBaseURL,
			email,
		); err != nil {
			// Deliberately keep the public response identical for existing and
			// unknown accounts, including delivery failures.
			log.Printf("request password reset: %v", err)
		}
		http.Redirect(writer, request, "/auth/forgot-password?requested=1", http.StatusSeeOther)
	default:
		writer.Header().Set("Allow", "GET, POST")
		http.Error(writer, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (server *Server) resetPassword(writer http.ResponseWriter, request *http.Request) {
	switch request.Method {
	case http.MethodGet:
		token := request.URL.Query().Get("token")
		valid := false
		var err error
		if token != "" {
			valid, err = db.PasswordResetTokenIsValid(server.Database, token)
			if err != nil {
				log.Printf("validate password reset token: %v", err)
				http.Error(writer, "Unable to validate password reset link", http.StatusInternalServerError)
				return
			}
		}
		server.render(writer, "reset-password.html", resetPasswordPageData{
			Token:   token,
			Invalid: !valid,
		})
	case http.MethodPost:
		request.Body = http.MaxBytesReader(writer, request.Body, 1<<20)
		if err := request.ParseForm(); err != nil {
			server.renderStatus(writer, http.StatusBadRequest, "reset-password.html", resetPasswordPageData{
				Error:   "Unable to process that request.",
				Invalid: true,
			})
			return
		}
		data := resetPasswordPageData{Token: request.FormValue("token")}
		password := request.FormValue("password")
		passwordConfirmation := request.FormValue("password_confirmation")
		if data.Token == "" || password == "" || passwordConfirmation == "" {
			data.Error = "The reset link and both password fields are required."
			server.renderStatus(writer, http.StatusBadRequest, "reset-password.html", data)
			return
		}
		if password != passwordConfirmation {
			data.Error = "The passwords do not match."
			server.renderStatus(writer, http.StatusBadRequest, "reset-password.html", data)
			return
		}
		if err := models.ValidatePassword(password); err != nil {
			switch {
			case errors.Is(err, models.ErrPasswordTooShort):
				data.Error = "Use at least 10 characters for your password."
			case errors.Is(err, models.ErrPasswordTooLong):
				data.Error = "Your password must be 72 bytes or fewer."
			}
			server.renderStatus(writer, http.StatusBadRequest, "reset-password.html", data)
			return
		}
		clientIP, err := ClientIP(request)
		if err != nil {
			log.Printf("resolve password reset attempt IP: %v", err)
			data.Error = "Unable to process that request."
			server.renderStatus(writer, http.StatusBadRequest, "reset-password.html", data)
			return
		}
		blocked, err := services.NewAuthenticationRateLimiter(server.Database).RegisterPasswordResetAttempt(clientIP.String())
		if err != nil {
			log.Printf("rate limit password reset attempt: %v", err)
			http.Error(writer, "Unable to reset password", http.StatusInternalServerError)
			return
		}
		if blocked {
			writer.Header().Set("Retry-After", "900")
			data.Error = "Too many attempts. Try again later."
			server.renderStatus(writer, http.StatusTooManyRequests, "reset-password.html", data)
			return
		}
		if err := services.ResetPassword(server.Database, data.Token, password); err != nil {
			if errors.Is(err, db.ErrInvalidPasswordResetToken) {
				data.Invalid = true
				data.Error = "This password reset link is invalid or has expired."
				server.renderStatus(writer, http.StatusBadRequest, "reset-password.html", data)
				return
			}
			log.Printf("reset password: %v", err)
			http.Error(writer, "Unable to reset password", http.StatusInternalServerError)
			return
		}
		http.Redirect(writer, request, "/auth/login?password-reset=1", http.StatusSeeOther)
	default:
		writer.Header().Set("Allow", "GET, POST")
		http.Error(writer, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (server *Server) logout(writer http.ResponseWriter, request *http.Request) {
	if !services.ValidCSRFToken(request, request.FormValue("csrf_token")) {
		http.Error(writer, "Invalid CSRF token", http.StatusForbidden)
		return
	}
	if cookie, err := request.Cookie(sessionCookieName); err == nil {
		if err := db.DeleteSession(server.Database, cookie.Value); err != nil {
			log.Printf("delete session: %v", err)
			http.Error(writer, "Unable to log out", http.StatusInternalServerError)
			return
		}
	}
	clearSessionCookie(writer, server.Config.IsProd())
	http.Redirect(writer, request, "/", http.StatusSeeOther)
}

func safeRedirectPath(value, fallback string) string {
	if value == "" || !strings.HasPrefix(value, "/") || strings.HasPrefix(value, "//") {
		return fallback
	}
	return value
}

func setSessionCookie(writer http.ResponseWriter, token string, secure bool, now time.Time) {
	http.SetCookie(writer, &http.Cookie{
		Name: sessionCookieName, Value: token, Path: "/",
		Expires: now.Add(db.SessionIdleTimeout), MaxAge: int(db.SessionIdleTimeout.Seconds()),
		HttpOnly: true, Secure: secure, SameSite: http.SameSiteLaxMode,
	})
}

func clearSessionCookie(writer http.ResponseWriter, secure bool) {
	http.SetCookie(writer, &http.Cookie{
		Name: sessionCookieName, Path: "/", Expires: time.Unix(0, 0), MaxAge: -1,
		HttpOnly: true, Secure: secure, SameSite: http.SameSiteLaxMode,
	})
}
