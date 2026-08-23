package http

import (
	"net/http"
	"time"

	"clever-consumer/backend/internal/config"
	"clever-consumer/backend/internal/http/controllers"
	"clever-consumer/backend/internal/http/middleware"
	"clever-consumer/backend/internal/services"
	"go.uber.org/zap"
)

func NewServer(cfg config.Config, app *services.Application, logger *zap.SugaredLogger) *http.Server {
	mux := http.NewServeMux()
	api := controllers.NewAPI(app, cfg, logger)

	mux.HandleFunc("GET /healthz", api.Health)
	mux.HandleFunc("GET /readyz", api.Ready)
	mux.HandleFunc("POST /v1/auth/magic-links", api.RequestMagicLink)
	mux.HandleFunc("POST /v1/auth/verify", api.VerifyMagicLink)
	mux.HandleFunc("GET /v1/auth/google/start", api.StartGoogleLogin)
	mux.HandleFunc("GET /v1/auth/google/callback", api.CompleteGoogleLogin)
	mux.HandleFunc("POST /v1/auth/logout", api.Logout)

	protected := middleware.RequireSession(cfg, app, logger)
	mux.Handle("GET /v1/me", protected(http.HandlerFunc(api.GetMe)))
	mux.Handle("PATCH /v1/me", protected(http.HandlerFunc(api.UpdateMe)))
	mux.Handle("POST /v1/product-previews", protected(http.HandlerFunc(api.CreatePreview)))
	mux.Handle("GET /v1/product-previews/{id}", protected(http.HandlerFunc(api.GetPreview)))
	mux.Handle("GET /v1/trackers", protected(http.HandlerFunc(api.ListTrackers)))
	mux.Handle("POST /v1/trackers", protected(http.HandlerFunc(api.CreateTracker)))
	mux.Handle("GET /v1/trackers/{id}", protected(http.HandlerFunc(api.GetTracker)))
	mux.Handle("DELETE /v1/trackers/{id}", protected(http.HandlerFunc(api.DeleteTracker)))
	mux.Handle("POST /v1/trackers/{id}/pause", protected(http.HandlerFunc(api.PauseTracker)))
	mux.Handle("POST /v1/trackers/{id}/resume", protected(http.HandlerFunc(api.ResumeTracker)))
	mux.Handle("POST /v1/trackers/{id}/run", protected(http.HandlerFunc(api.RunTracker)))
	mux.Handle("GET /v1/trackers/{id}/observations", protected(http.HandlerFunc(api.Observations)))
	mux.Handle("GET /v1/scraper/collectors", protected(http.HandlerFunc(api.ListCollectors)))
	mux.Handle("POST /v1/scraper/collectors/{id}/heal", protected(http.HandlerFunc(api.HealCollector)))

	handler := middleware.CORS(middleware.Correlation(middleware.RequestLogger(logger)(mux)))
	return &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
	}
}
