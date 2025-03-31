package server

import (
	"fmt"
	"net/http"

	"github.com/bezerker/sndbot/bot"
)

// StartCallbackServer starts a server to handle OAuth callbacks
func StartCallbackServer(port int) error {
	http.HandleFunc("/oauth/callback", handleCallback)
	return http.ListenAndServe(fmt.Sprintf(":%d", port), nil)
}

func handleCallback(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get code and state from query parameters
	code := r.URL.Query().Get("code")
	state := r.URL.Query().Get("state")

	if code == "" || state == "" {
		http.Error(w, "Missing required parameters", http.StatusBadRequest)
		return
	}

	// Handle the OAuth callback
	err := bot.HandleOAuthCallback(code, state)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to handle OAuth callback: %v", err), http.StatusInternalServerError)
		return
	}

	// Return success page
	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(`
		<!DOCTYPE html>
		<html>
		<head>
			<title>Authentication Successful</title>
			<style>
				body {
					font-family: Arial, sans-serif;
					display: flex;
					justify-content: center;
					align-items: center;
					height: 100vh;
					margin: 0;
					background-color: #f0f0f0;
				}
				.container {
					text-align: center;
					padding: 2rem;
					background-color: white;
					border-radius: 8px;
					box-shadow: 0 2px 4px rgba(0,0,0,0.1);
				}
				h1 {
					color: #2c3e50;
					margin-bottom: 1rem;
				}
				p {
					color: #7f8c8d;
				}
			</style>
		</head>
		<body>
			<div class="container">
				<h1>Authentication Successful!</h1>
				<p>You can now close this window and return to Discord.</p>
			</div>
		</body>
		</html>
	`))
}
