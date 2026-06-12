package main

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"time"

	"cardgame.ryanharris.net/internal/data"
	"github.com/coreos/go-oidc/v3/oidc"
)

func randString(nByte int) (string, error) {
	b := make([]byte, nByte)
	if _, err := io.ReadFull(rand.Reader, b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func setCallbackCookie(w http.ResponseWriter, r *http.Request, name, value string) {
	c := &http.Cookie{
		Name:     name,
		Value:    value,
		Path:     "/",
		MaxAge:   int(time.Hour.Seconds()),
		Secure:   r.TLS != nil,
		HttpOnly: true,
	}
	http.SetCookie(w, c)
}

func (app *application) login(w http.ResponseWriter, r *http.Request) {

	state, err := randString(16)
	if err != nil {
		http.Error(w, "Internal error", http.StatusInternalServerError)
		return
	}
	nonce, err := randString(16)
	if err != nil {
		http.Error(w, "Internal error", http.StatusInternalServerError)
		return
	}
	setCallbackCookie(w, r, "state", state)
	setCallbackCookie(w, r, "nonce", nonce)

	http.Redirect(w, r, app.auth.oauthConfig.AuthCodeURL(state, oidc.Nonce(nonce)), http.StatusFound)
}

func (app *application) signin(w http.ResponseWriter, r *http.Request) {
	state, err := r.Cookie("state")
	fmt.Println(r.Header.Get("Cookie"))
	if err != nil {
		http.Error(w, "state not found", http.StatusBadRequest)
		return
	}
	if r.URL.Query().Get("state") != state.Value {
		http.Error(w, "state did not match", http.StatusBadRequest)
		return
	}

	ctx := r.Context()
	oauth2Token, err := app.auth.oauthConfig.Exchange(ctx, r.URL.Query().Get("code"))
	if err != nil {
		http.Error(w, "Failed to exchange token: "+err.Error(), http.StatusInternalServerError)
		return
	}
	rawIDToken, ok := oauth2Token.Extra("id_token").(string)
	if !ok {
		http.Error(w, "No id_token field in oauth2 token.", http.StatusInternalServerError)
		return
	}
	idToken, err := app.auth.verifier.Verify(ctx, rawIDToken)
	if err != nil {
		http.Error(w, "Failed to verify ID Token: "+err.Error(), http.StatusInternalServerError)
		return
	}

	nonce, err := r.Cookie("nonce")
	if err != nil {
		http.Error(w, "nonce not found", http.StatusBadRequest)
		return
	}
	if idToken.Nonce != nonce.Value {
		http.Error(w, "nonce did not match", http.StatusBadRequest)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "state",
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
	})

	http.SetCookie(w, &http.Cookie{
		Name:     "nonce",
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
	})

	oauth2Token.AccessToken = "*REDACTED*"

	var user data.User

	if err := idToken.Claims(&user); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	fmt.Printf("%v\n", user)

	user.Username = ""

	existing_user, err := app.models.Users.GetBySub(user.Sub)
	if err != nil {
		app.logger.Error("Database lookup failed in signin", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	if existing_user == nil {
		err = app.models.Users.Insert(&user)
		if err != nil {
			app.logger.Error("Database insert failed in sign in", "error", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}
	} else {
		user = *existing_user
	}

	app.sessionManager.Put(r.Context(), "authenticatedUserID", user.ID)

}
