package main

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
)

// WebfingerResponse represents a webfinger JSON response
type WebfingerResponse struct {
	Subject string `json:"subject"`
	Links   []Link `json:"links"`
}

// Link represents a link in a webfinger response
type Link struct {
	Rel  string `json:"rel"`
	Href string `json:"href"`
}

// webfingerHandler handles webfinger requests
func webfingerHandler(w http.ResponseWriter, r *http.Request) {
	email := os.Getenv("EMAIL_ADDRESS")

	response := WebfingerResponse{
		Subject: "acct:" + email,
		Links: []Link{
			{
				Rel:  "http://openid.net/specs/connect/1.0/issuer",
				Href: "https://id.lds.li",
			},
		},
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		slog.Error("Failed to encode webfinger response", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
}
