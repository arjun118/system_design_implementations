package main

import (
	"embed"
	"encoding/json"
	"flag"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/arjun118/autocomplete/internal/trie"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

//go:embed static
var staticFS embed.FS

// suggester is the serving structure, loaded once at boot. Both
// implementations (trie, prefix-hash) are read-only after Load, so concurrent
// handlers are safe.
var suggester trie.Suggester

type suggestResponse struct {
	Prefix      string   `json:"prefix"`
	Suggestions []string `json:"suggestions"`
	DurationNS  int64    `json:"duration_ns"`
}

type errorResponse struct {
	Error string `json:"error"`
}

func HandleIndex(w http.ResponseWriter, r *http.Request) {
	html, err := staticFS.ReadFile("static/index.html")
	if err != nil {
		http.Error(w, "index.html not found", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if _, err := w.Write(html); err != nil {
		log.Printf("write index: %v", err)
	}
}

func HandleSuggest(w http.ResponseWriter, r *http.Request) {
	// Primary param is q (README spec); accept "search" as an alias.
	q := r.URL.Query().Get("q")
	if q == "" {
		q = r.URL.Query().Get("search")
	}
	q = strings.TrimSpace(q)
	if q == "" {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "missing query parameter: q"})
		return
	}

	start := time.Now()
	res := suggester.Suggest(q)
	elapsed := time.Since(start)
	if res == nil {
		res = []string{} // serialize as [] not null
	}
	// expose the suggest time to non-browser callers too
	w.Header().Set("X-Suggest-Time-Ns", strconv.FormatInt(elapsed.Nanoseconds(), 10))

	// Optional per-request cap. The cache is built with the boot-time K, so a
	// smaller k here only trims the returned slice.
	if kStr := r.URL.Query().Get("k"); kStr != "" {
		if k, err := strconv.Atoi(kStr); err == nil && k >= 0 && k < len(res) {
			res = res[:k]
		}
	}

	writeJSON(w, http.StatusOK, suggestResponse{Prefix: q, Suggestions: res, DurationNS: elapsed.Nanoseconds()})
}

func HandleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("write response: %v", err)
	}
}

// corsMiddleware keeps the static UI usable from a different origin (Phase 6).
func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func main() {
	var (
		file = flag.String("file", "/home/cicada/system_design_implementations/search_autocomplete/aol_queries.txt",
			"data file (phrase TAB freq)")
		impl = flag.String("impl", "trie", "implementation: trie | prefix-hash")
		k    = flag.Int("k", 10, "cache size (top-k suggestions)")
		addr = flag.String("addr", ":8080", "listen address")
	)
	flag.Parse()

	s, err := trie.Load(*file, *impl, *k)
	if err != nil {
		log.Fatalf("load %s (%s): %v", *file, *impl, err)
	}
	suggester = s
	log.Printf("loaded suggester: impl=%s k=%d", *impl, *k)

	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(corsMiddleware)

	r.Get("/", HandleIndex)
	r.Get("/healthz", HandleHealth)
	r.Get("/api/suggest", HandleSuggest)

	log.Printf("server listening on %s", *addr)
	if err := http.ListenAndServe(*addr, r); err != nil {
		log.Fatalf("server: %v", err)
	}
}
