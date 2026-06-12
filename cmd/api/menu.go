package main

import "net/http"

func (app *application) pregameMenu(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte("Pregame Menu"))
}
