package api

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"regexp"
	"strings"

	"beerbot-go/internal/decision"
)

// Spec reminders:
// - POST only
// - Content-Type: application/json
// - Handshake response must include ok, student_email (@taltech.ee), algorithm_name, version, supports, message="BeerBot ready"
// - Weekly response must include orders for all 4 roles, non-negative integers
// :contentReference[oaicite:1]{index=1}

type Config struct {
	StudentEmail  string
	AlgorithmName string
	Version       string

	SupportsBlackBox bool
	SupportsGlassBox bool

	// BlackBox tuning knobs (safe defaults)
	SafetyStock int // e.g. +10
	MAWindow    int // moving average window for demand smoothing (>=1)
	MaxOrder    int // 0 means "no cap"
}

func ConfigFromEnv() Config {
	cfg := Config{
		StudentEmail:     getenv("STUDENT_EMAIL", "firstname.lastname@taltech.ee"),
		AlgorithmName:    getenv("ALGORITHM_NAME", "BeerBot_BlackBox"),
		Version:          getenv("VERSION", "v1.0.0"),
		SupportsBlackBox: true,
		SupportsGlassBox: false,
		SafetyStock:      getenvInt("SAFETY_STOCK", 10),
		MAWindow:         getenvInt("MA_WINDOW", 4),
		MaxOrder:         getenvInt("MAX_ORDER", 0),
	}
	return cfg
}

func (c Config) Validate() error {
	if !strings.HasSuffix(strings.ToLower(c.StudentEmail), "@taltech.ee") {
		return errors.New("STUDENT_EMAIL must end with @taltech.ee")
	}
	// algorithm_name: 3–32 chars, letters/digits/underscores
	re := regexp.MustCompile(`^[A-Za-z0-9_]{3,32}$`)
	if !re.MatchString(c.AlgorithmName) {
		log.Printf("ALGORITHM_NAME does not match %s", c.AlgorithmName)
		return errors.New("ALGORITHM_NAME must be 3–32 chars: letters/digits/underscores only")
	}
	// version: "v1" or "v1.2.3" style (lenient)
	if !strings.HasPrefix(c.Version, "v") {
		return errors.New("VERSION should look like v1, v1.0 or v1.2.3")
	}
	if !(c.SupportsBlackBox || c.SupportsGlassBox) {
		return errors.New("at least one of SupportsBlackBox or SupportsGlassBox must be true")
	}
	if c.MAWindow < 1 {
		return errors.New("MA_WINDOW must be >= 1")
	}
	if c.SafetyStock < 0 {
		return errors.New("SAFETY_STOCK must be >= 0")
	}
	if c.MaxOrder < 0 {
		return errors.New("MAX_ORDER must be >= 0")
	}
	return nil
}

type handshakeRequest struct {
	Handshake bool   `json:"handshake"`
	Ping      string `json:"ping"`
	Seed      int    `json:"seed"`
}

type weeklyRequest struct {
	Mode       string               `json:"mode"` // "blackbox" or "glassbox"
	Week       int                  `json:"week"`
	WeeksTotal int                  `json:"weeks_total"` // typically 36
	Seed       int                  `json:"seed"`
	Weeks      []decision.WeekState `json:"weeks"`
	// NOTE: spec may add optional fields; we ignore unknown fields safely.
}

type handshakeResponse struct {
	Ok            bool              `json:"ok"`
	StudentEmail  string            `json:"student_email"`
	AlgorithmName string            `json:"algorithm_name"`
	Version       string            `json:"version"`
	Supports      map[string]bool   `json:"supports"`
	Message       string            `json:"message"`
	Optional      map[string]string `json:"-"`
}

type weeklyResponse struct {
	Orders map[string]int `json:"orders"`
}

func NewDecisionHandler(cfg Config) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Always respond JSON, always HTTP 200 per spec expectations. :contentReference[oaicite:2]{index=2}
		w.Header().Set("Content-Type", "application/json")

		if r.Method != http.MethodPost {
			// Return a safe JSON so simulator gets valid JSON.
			_ = json.NewEncoder(w).Encode(map[string]any{
				"error": "POST required",
				"orders": map[string]int{
					"retailer":    10,
					"wholesaler":  10,
					"distributor": 10,
					"factory":     10,
				},
			})
			return
		}

		// Decode into a generic envelope to detect handshake without failing on weekly fields.
		var env struct {
			Handshake *bool `json:"handshake"`
		}
		dec := json.NewDecoder(r.Body)
		if err := dec.Decode(&env); err != nil {
			_ = json.NewEncoder(w).Encode(defaultOrders())
			return
		}

		if env.Handshake != nil && *env.Handshake {
			resp := handshakeResponse{
				Ok:            true,
				StudentEmail:  cfg.StudentEmail,
				AlgorithmName: cfg.AlgorithmName,
				Version:       cfg.Version,
				Supports: map[string]bool{
					"blackbox": cfg.SupportsBlackBox,
					"glassbox": cfg.SupportsGlassBox,
				},
				Message: "BeerBot ready", // must match exactly :contentReference[oaicite:3]{index=3}
			}
			_ = json.NewEncoder(w).Encode(resp)
			return
		}

		// Re-decode (we consumed body already). In real servers we'd buffer; easiest: require client sends once.
		// To keep this handler simple and robust, we instead recommend setting a MaxBytesReader and decoding once.
		// Practically, most Go servers can decode into weeklyRequest directly by decoding the whole body once.
		//
		// We'll solve it by reading the raw body first in a small wrapper (see below).
		// This branch is handled in NewDecisionHandlerBuffered.
		_ = json.NewEncoder(w).Encode(map[string]any{"error": "internal: use buffered handler"})
	})
}

// NewDecisionHandlerBuffered is the actual handler used by main.go.
// It reads the body bytes once so we can decode twice (handshake detection + weekly request).
func NewDecisionHandlerBuffered(cfg Config) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		if r.Method != http.MethodPost {
			_ = json.NewEncoder(w).Encode(defaultOrders())
			return
		}

		body, err := readAllLimited(r, 1<<20) // 1MB max
		if err != nil {
			_ = json.NewEncoder(w).Encode(defaultOrders())
			return
		}

		var hs handshakeRequest
		_ = json.Unmarshal(body, &hs)
		if hs.Handshake {
			log.Printf("[HANDSHAKE] ping=%q seed=%d", hs.Ping, hs.Seed)
			resp := handshakeResponse{
				Ok:            true,
				StudentEmail:  cfg.StudentEmail,
				AlgorithmName: cfg.AlgorithmName,
				Version:       cfg.Version,
				Supports: map[string]bool{
					"blackbox": cfg.SupportsBlackBox,
					"glassbox": cfg.SupportsGlassBox,
				},
				Message: "BeerBot ready",
			}
			_ = json.NewEncoder(w).Encode(resp)
			return
		}

		var req weeklyRequest
		if err := json.Unmarshal(body, &req); err != nil || len(req.Weeks) == 0 {
			_ = json.NewEncoder(w).Encode(defaultOrders())
			return
		}
		log.Printf("[WEEK] mode=%s week=%d/%d seed=%d weeks_len=%d",
			req.Mode, req.Week, req.WeeksTotal, req.Seed, len(req.Weeks))

		// We only support blackbox decisions, but we can still accept mode field.
		// In blackbox, each role order depends only on that role's own history.
		orders := make(map[string]int, 4)
		for _, role := range []string{"retailer", "wholesaler", "distributor", "factory"} {
			history := decision.ExtractRoleHistory(req.Weeks, role)
			log.Printf("role=%s history_len=%d", role, len(history))

			ordersHist := decision.ExtractRoleOrders(req.Weeks, role)
			o := decision.BlackBoxOrderWithPipeline(history, ordersHist, cfg.SafetyStock, cfg.MAWindow)
			if cfg.MaxOrder > 0 && o > cfg.MaxOrder {
				o = cfg.MaxOrder
			}
			orders[role] = o

			last := history[len(history)-1]
			log.Printf("[ROLE] %s inv=%d back=%d in_orders=%d arriving=%d -> order=%d",
				role, last.Inventory, last.Backlog, last.IncomingOrders, last.ArrivingShipments, o)
		}

		_ = json.NewEncoder(w).Encode(weeklyResponse{Orders: orders})
	})
}

// NOTE: main.go should register the buffered handler.
// Replace in main.go: mux.Handle("/api/decision", api.NewDecisionHandler(cfg))
// with: mux.Handle("/api/decision", api.NewDecisionHandlerBuffered(cfg))

func defaultOrders() weeklyResponse {
	return weeklyResponse{Orders: map[string]int{
		"retailer":    10,
		"wholesaler":  10,
		"distributor": 10,
		"factory":     10,
	}}
}
