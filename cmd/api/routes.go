package main

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

func (app *application) routes() http.Handler {

	router := chi.NewRouter()

	router.Route("/v1/auth", func(r chi.Router) {
		r.Use(app.rateLimit)
		r.Use(app.sessionManager.LoadAndSave)

		r.Get("/login", app.login)
		r.Get("/google/callback", app.signin)
	})

	router.Route("/v1/api", func(r chi.Router) {
		r.Use(app.rateLimit)
		r.Use(app.sessionManager.LoadAndSave)
		r.Use(app.requireAuthentication)
		r.Get("/pregame", app.pregameMenu)

	})

	router.Route("/v1/game", func(r chi.Router) {
		r.Use(app.sessionManager.LoadAndSave)
		r.With(app.limitActiveWebSockets).Get("/ws", app.serveWS)
	})

	router.Get("/v1/healthcheck", app.healthcheckHandler)

	return router
}
