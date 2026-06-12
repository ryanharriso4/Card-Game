package main

import "net/http"

func (app *application) rateLimitExceededResponse(w http.ResponseWriter, r *http.Request) {
	app.logger.Error("rate limit exceeded")
	http.Error(w, "rate limit exceeded", http.StatusTooManyRequests)
}
