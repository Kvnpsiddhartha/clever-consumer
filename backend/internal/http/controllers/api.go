package controllers

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"clever-consumer/backend/internal/config"
	"clever-consumer/backend/internal/domain"
	"clever-consumer/backend/internal/http/middleware"
	"clever-consumer/backend/internal/services"
	"go.uber.org/zap"
)

const googleOAuthStateCookie = "cc_google_oauth_state"

type API struct {
	app    *services.Application
	cfg    config.Config
	logger *zap.SugaredLogger
}

func NewAPI(app *services.Application, cfg config.Config, logger *zap.SugaredLogger) *API {
	return &API{app: app, cfg: cfg, logger: logger}
}

type errorEnvelope struct {
	Code          string            `json:"code"`
	Message       string            `json:"message"`
	FieldErrors   map[string]string `json:"field_errors,omitempty"`
	CorrelationID string            `json:"correlation_id"`
}

func (a *API) Health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (a *API) Ready(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
}

func (a *API) RequestMagicLink(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email string `json:"email"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	link, err := a.app.RequestMagicLink(r.Context(), req.Email)
	if err != nil {
		writeError(w, r, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]any{
		"id":             link.ID,
		"expires_at":     link.ExpiresAt,
		"dev_magic_link": a.cfg.PublicBaseURL + "/auth/verify?token=" + link.Token,
	})
}

func (a *API) VerifyMagicLink(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Token string `json:"token"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	user, sessionID, expiresAt, err := a.app.VerifyMagicLink(r.Context(), req.Token)
	if err != nil {
		writeError(w, r, http.StatusUnauthorized, "invalid_token", "magic link is invalid or expired")
		return
	}
	a.setSessionCookie(w, sessionID, expiresAt)
	writeJSON(w, http.StatusOK, user)
}

func (a *API) StartGoogleLogin(w http.ResponseWriter, r *http.Request) {
	if a.cfg.GoogleClientID == "" || a.cfg.GoogleClientSecret == "" {
		a.redirectAuthError(w, r, "google_auth_not_configured")
		return
	}
	state := randomState()
	http.SetCookie(w, &http.Cookie{
		Name:     googleOAuthStateCookie,
		Value:    state,
		Path:     "/v1/auth/google",
		Expires:  time.Now().UTC().Add(10 * time.Minute),
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   strings.HasPrefix(a.cfg.GoogleRedirectURL, "https://"),
	})
	params := url.Values{}
	params.Set("client_id", a.cfg.GoogleClientID)
	params.Set("redirect_uri", a.cfg.GoogleRedirectURL)
	params.Set("response_type", "code")
	params.Set("scope", "openid email profile")
	params.Set("state", state)
	params.Set("prompt", "select_account")
	http.Redirect(w, r, "https://accounts.google.com/o/oauth2/v2/auth?"+params.Encode(), http.StatusFound)
}

func (a *API) CompleteGoogleLogin(w http.ResponseWriter, r *http.Request) {
	if oauthErr := r.URL.Query().Get("error"); oauthErr != "" {
		a.redirectAuthError(w, r, oauthErr)
		return
	}
	state := r.URL.Query().Get("state")
	stateCookie, err := r.Cookie(googleOAuthStateCookie)
	if err != nil || state == "" || subtle.ConstantTimeCompare([]byte(state), []byte(stateCookie.Value)) != 1 {
		a.redirectAuthError(w, r, "invalid_state")
		return
	}
	code := r.URL.Query().Get("code")
	if code == "" {
		a.redirectAuthError(w, r, "missing_code")
		return
	}
	email, err := a.googleEmail(r, code)
	if err != nil {
		a.logger.Warnw("google sign-in failed", "error", err)
		a.redirectAuthError(w, r, "google_sign_in_failed")
		return
	}
	_, sessionID, expiresAt, err := a.app.CreateEmailSession(r.Context(), email)
	if err != nil {
		a.logger.Warnw("google session creation failed", "error", err)
		a.redirectAuthError(w, r, "session_failed")
		return
	}
	a.clearGoogleStateCookie(w)
	a.setSessionCookie(w, sessionID, expiresAt)
	http.Redirect(w, r, a.cfg.PublicBaseURL, http.StatusFound)
}

func (a *API) Logout(w http.ResponseWriter, _ *http.Request) {
	http.SetCookie(w, &http.Cookie{Name: a.cfg.SessionCookieName, Value: "", Path: "/", Expires: time.Unix(0, 0), HttpOnly: true})
	w.WriteHeader(http.StatusNoContent)
}

func (a *API) GetMe(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, middleware.User(r))
}

func (a *API) UpdateMe(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Country  string `json:"country"`
		Timezone string `json:"timezone"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	user := middleware.User(r)
	updated, err := a.app.UpdateProfile(r.Context(), user.ID, req.Country, req.Timezone)
	if err != nil {
		writeError(w, r, http.StatusBadRequest, "invalid_profile", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, updated)
}

func (a *API) CreatePreview(w http.ResponseWriter, r *http.Request) {
	var req struct {
		URL     string `json:"url"`
		Country string `json:"country"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	preview, err := a.app.CreatePreview(r.Context(), middleware.User(r), req.URL, req.Country)
	if err != nil {
		writeError(w, r, http.StatusBadRequest, "preview_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusAccepted, preview)
}

func (a *API) GetPreview(w http.ResponseWriter, r *http.Request) {
	preview, err := a.app.Preview(r.Context(), middleware.User(r).ID, r.PathValue("id"))
	if err != nil {
		writeError(w, r, http.StatusNotFound, "not_found", "preview not found")
		return
	}
	writeJSON(w, http.StatusOK, preview)
}

func (a *API) CreateTracker(w http.ResponseWriter, r *http.Request) {
	var req struct {
		PreviewID     string             `json:"preview_id"`
		IntervalHours int                `json:"interval_hours"`
		Country       string             `json:"country"`
		Rules         []domain.AlertRule `json:"rules"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	tracker, err := a.app.CreateTracker(r.Context(), middleware.User(r).ID, req.PreviewID, req.IntervalHours, req.Country, req.Rules)
	if err != nil {
		writeError(w, r, http.StatusBadRequest, "tracker_create_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, tracker)
}

func (a *API) ListTrackers(w http.ResponseWriter, r *http.Request) {
	trackers, err := a.app.ListTrackers(r.Context(), middleware.User(r).ID)
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "tracker_list_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": trackers})
}

func (a *API) GetTracker(w http.ResponseWriter, r *http.Request) {
	tracker, err := a.app.Tracker(r.Context(), middleware.User(r).ID, r.PathValue("id"))
	if err != nil {
		writeError(w, r, http.StatusNotFound, "not_found", "tracker not found")
		return
	}
	writeJSON(w, http.StatusOK, tracker)
}

func (a *API) DeleteTracker(w http.ResponseWriter, r *http.Request) {
	if err := a.app.DeleteTracker(r.Context(), middleware.User(r).ID, r.PathValue("id")); err != nil {
		writeError(w, r, http.StatusNotFound, "not_found", "tracker not found")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *API) PauseTracker(w http.ResponseWriter, r *http.Request) {
	if err := a.app.SetTrackerStatus(r.Context(), middleware.User(r).ID, r.PathValue("id"), domain.TrackerPaused); err != nil {
		writeError(w, r, http.StatusNotFound, "not_found", "tracker not found")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *API) ResumeTracker(w http.ResponseWriter, r *http.Request) {
	if err := a.app.SetTrackerStatus(r.Context(), middleware.User(r).ID, r.PathValue("id"), domain.TrackerActive); err != nil {
		writeError(w, r, http.StatusNotFound, "not_found", "tracker not found")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *API) RunTracker(w http.ResponseWriter, r *http.Request) {
	obs, err := a.app.RunTrackerNow(r.Context(), middleware.User(r).ID, r.PathValue("id"))
	if err != nil {
		writeError(w, r, http.StatusBadRequest, "manual_run_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusAccepted, obs)
}

func (a *API) Observations(w http.ResponseWriter, r *http.Request) {
	items, err := a.app.Observations(r.Context(), middleware.User(r).ID, r.PathValue("id"))
	if err != nil {
		writeError(w, r, http.StatusNotFound, "not_found", "tracker not found")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func decodeJSON(w http.ResponseWriter, r *http.Request, target any) bool {
	defer r.Body.Close()
	if err := json.NewDecoder(r.Body).Decode(target); err != nil {
		writeError(w, r, http.StatusBadRequest, "invalid_json", "request body must be valid JSON")
		return false
	}
	return true
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, r *http.Request, status int, code, message string) {
	writeJSON(w, status, errorEnvelope{Code: code, Message: message, CorrelationID: middleware.CorrelationID(r)})
}

func (a *API) googleEmail(r *http.Request, code string) (string, error) {
	token, err := a.exchangeGoogleCode(r, code)
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, "https://www.googleapis.com/oauth2/v3/userinfo", nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	response, err := (&http.Client{Timeout: 10 * time.Second}).Do(req)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if err != nil {
		return "", err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return "", errors.New("google userinfo returned " + response.Status)
	}
	var info struct {
		Email         string `json:"email"`
		EmailVerified bool   `json:"email_verified"`
	}
	if err := json.Unmarshal(body, &info); err != nil {
		return "", err
	}
	if !info.EmailVerified {
		return "", errors.New("google email is not verified")
	}
	email := strings.ToLower(strings.TrimSpace(info.Email))
	if !strings.Contains(email, "@") {
		return "", errors.New("google userinfo response did not include a valid email")
	}
	return email, nil
}

func (a *API) exchangeGoogleCode(r *http.Request, code string) (string, error) {
	form := url.Values{}
	form.Set("client_id", a.cfg.GoogleClientID)
	form.Set("client_secret", a.cfg.GoogleClientSecret)
	form.Set("code", code)
	form.Set("grant_type", "authorization_code")
	form.Set("redirect_uri", a.cfg.GoogleRedirectURL)
	req, err := http.NewRequestWithContext(r.Context(), http.MethodPost, "https://oauth2.googleapis.com/token", strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response, err := (&http.Client{Timeout: 10 * time.Second}).Do(req)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if err != nil {
		return "", err
	}
	var tokenResponse struct {
		AccessToken      string `json:"access_token"`
		Error            string `json:"error"`
		ErrorDescription string `json:"error_description"`
	}
	if err := json.Unmarshal(body, &tokenResponse); err != nil {
		return "", err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		if tokenResponse.ErrorDescription != "" {
			return "", errors.New(tokenResponse.ErrorDescription)
		}
		if tokenResponse.Error != "" {
			return "", errors.New(tokenResponse.Error)
		}
		return "", errors.New("google token endpoint returned " + response.Status)
	}
	if tokenResponse.AccessToken == "" {
		return "", errors.New("google token response did not include an access token")
	}
	return tokenResponse.AccessToken, nil
}

func (a *API) setSessionCookie(w http.ResponseWriter, sessionID string, expiresAt time.Time) {
	http.SetCookie(w, &http.Cookie{
		Name:     a.cfg.SessionCookieName,
		Value:    sessionID,
		Path:     "/",
		Expires:  expiresAt,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   strings.HasPrefix(a.cfg.PublicBaseURL, "https://"),
	})
}

func (a *API) clearGoogleStateCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     googleOAuthStateCookie,
		Value:    "",
		Path:     "/v1/auth/google",
		Expires:  time.Unix(0, 0),
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   strings.HasPrefix(a.cfg.GoogleRedirectURL, "https://"),
	})
}

func (a *API) redirectAuthError(w http.ResponseWriter, r *http.Request, code string) {
	a.clearGoogleStateCookie(w)
	target, err := url.Parse(a.cfg.PublicBaseURL)
	if err != nil {
		writeError(w, r, http.StatusUnauthorized, "auth_failed", "authentication failed")
		return
	}
	query := target.Query()
	query.Set("auth_error", code)
	target.RawQuery = query.Encode()
	http.Redirect(w, r, target.String(), http.StatusFound)
}

func randomState() string {
	var b [32]byte
	_, _ = rand.Read(b[:])
	return base64.RawURLEncoding.EncodeToString(b[:])
}
