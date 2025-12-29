package main

import (
	"log"
	"net/http"
	"os"
	"time"

	"beerbot-go/internal/api"
)

func main() {
	cfg := api.ConfigFromEnv()
	if err := cfg.Validate(); err != nil {
		log.Fatalf("config invalid: %v", err)
	}

	mux := http.NewServeMux()
	mux.Handle("/api/decision", api.NewDecisionHandlerBuffered(cfg))

	// Optional health endpoint (handy for Render checks)
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	srv := &http.Server{
		Addr:              ":" + port,
		Handler:           mux,
		ReadHeaderTimeout: 2 * time.Second,
		ReadTimeout:       2 * time.Second,
		WriteTimeout:      2 * time.Second,
		IdleTimeout:       30 * time.Second,
	}

	log.Printf("BeerBot listening on :%s (algo=%s, version=%s)", port, cfg.AlgorithmName, cfg.Version)
	log.Fatal(srv.ListenAndServe())
}
