package controllers

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"clever-consumer/backend/internal/config"
	"clever-consumer/backend/internal/domain"
	"clever-consumer/backend/internal/http/middleware"
	"clever-consumer/backend/internal/services"
)

type API struct {
	app    *services.Application
	cfg    config.Config
	logger *slog.Logger
}

func NewAPI(app *services.Application, cfg config.Config, logger *slog.Logger) *API {
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
	http.SetCookie(w, &http.Cookie{
		Name:     a.cfg.SessionCookieName,
		Value:    sessionID,
		Path:     "/",
		Expires:  expiresAt,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   strings.HasPrefix(a.cfg.PublicBaseURL, "https://"),
	})
	writeJSON(w, http.StatusOK, user)
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
