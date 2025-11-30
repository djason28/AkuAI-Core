package main

import (
	"context"
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"AkuAI/pkg/config"
	svc "AkuAI/pkg/services"
)

// context keys for prompt logging
type ctxKey string

const (
	ctxRunID   ctxKey = "abtest_run_id"
	ctxMode    ctxKey = "abtest_mode"
	ctxLogFile ctxKey = "abtest_log_file"
	ctxLogFull ctxKey = "abtest_log_full"
)

type QueryItem struct {
	Q string `json:"q"`
}

type ResultItem struct {
	Query                 string   `json:"query"`
	Mode                  string   `json:"mode"` // baseline | engineered
	Response              string   `json:"response"`
	Error                 string   `json:"error,omitempty"`
	DurationMs            int64    `json:"duration_ms"`
	Model                 string   `json:"model"`
	Timestamp             string   `json:"timestamp"`
	PromptTemplateID      string   `json:"prompt_template_id,omitempty"`
	PromptTemplateVersion string   `json:"prompt_template_version,omitempty"`
	ContextHash           string   `json:"context_hash,omitempty"`
	ContextSnapshot       string   `json:"context_snapshot,omitempty"`
	RelevantEventIDs      []string `json:"relevant_event_ids,omitempty"`
}

type RunSummary struct {
	RunID        string       `json:"run_id"`
	RandomSeed   int64        `json:"random_seed"`
	StartedAt    string       `json:"started_at"`
	EndedAt      string       `json:"ended_at"`
	Env          string       `json:"env"`
	GeminiOn     bool         `json:"gemini_enabled"`
	Model        string       `json:"model"`
	Temperature  float64      `json:"temperature"`
	ABTestOnly   string       `json:"abtest_only,omitempty"`
	PromptLog    string       `json:"prompt_log_file,omitempty"`
	TotalQueries int          `json:"total_queries"`
	Results      []ResultItem `json:"results"`
}

func mustReadQueries() ([]string, error) {
	// Try multiple relative locations to be robust when called via `go run ./core/cmd/abtest`
	candidates := []string{
		"core/cmd/abtest/queries.json",
		"cmd/abtest/queries.json",
		"queries.json",
		filepath.Join(filepath.Dir(os.Args[0]), "queries.json"),
	}

	var data []byte
	var err error
	for _, p := range candidates {
		if b, e := os.ReadFile(p); e == nil {
			data = b
			err = nil
			break
		} else {
			err = e
		}
	}
	if data == nil {
		return nil, fmt.Errorf("cannot read queries.json: %w", err)
	}

	// queries.json can be either ["q1", "q2", ...] or [{"q": "..."}, ...]
	var arrAny []any
	if e := json.Unmarshal(data, &arrAny); e != nil {
		return nil, fmt.Errorf("invalid queries.json: %w", e)
	}
	out := make([]string, 0, len(arrAny))
	for _, v := range arrAny {
		switch t := v.(type) {
		case string:
			out = append(out, strings.TrimSpace(t))
		case map[string]any:
			if qv, ok := t["q"].(string); ok {
				out = append(out, strings.TrimSpace(qv))
			}
		}
	}
	if len(out) == 0 {
		return nil, errors.New("queries.json is empty or malformed")
	}
	return out, nil
}

func ensureDir(p string) error {
	return os.MkdirAll(p, 0o755)
}

func writeJSON(path string, v any) error {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o644)
}

func writeCSV(path string, items []ResultItem) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	w := csv.NewWriter(f)
	defer w.Flush()
	// header
	_ = w.Write([]string{"query", "mode", "duration_ms", "model", "error", "response"})
	for _, it := range items {
		_ = w.Write([]string{
			it.Query,
			it.Mode,
			fmt.Sprintf("%d", it.DurationMs),
			it.Model,
			it.Error,
			it.Response,
		})
	}
	return nil
}

func main() {
	// Trigger config init() to load env
	_ = config.AppEnv

	if !config.IsGeminiEnabled {
		fmt.Println("[warn] IS_GEMINI_ENABLED=0 – runner will use mock responses. Enable real API for valid A/B results.")
	}
	if config.GeminiAPIKey == "" {
		fmt.Println("[warn] GEMINI_API_KEY is empty – real API calls will fail. Set it in core/.env")
	}

	queries, err := mustReadQueries()
	if err != nil {
		fmt.Println("error:", err)
		os.Exit(1)
	}

	// Optional: run only selected queries by index or substring via ABTEST_ONLY
	// Example: ABTEST_ONLY="10,11,14" or ABTEST_ONLY="minggu depan,bulan 11"
	if only := strings.TrimSpace(os.Getenv("ABTEST_ONLY")); only != "" {
		tokens := strings.Split(only, ",")
		wantedIdx := map[int]bool{}
		subs := make([]string, 0)
		for _, t := range tokens {
			v := strings.ToLower(strings.TrimSpace(t))
			if v == "" {
				continue
			}
			if n, err := strconv.Atoi(v); err == nil {
				if n >= 1 && n <= len(queries) {
					wantedIdx[n-1] = true
				}
			} else {
				subs = append(subs, v)
			}
		}
		filtered := make([]string, 0)
		seen := map[int]bool{}
		for i := range queries {
			if wantedIdx[i] {
				filtered = append(filtered, queries[i])
				seen[i] = true
			}
		}
		if len(subs) > 0 {
			for i, q := range queries {
				if seen[i] {
					continue
				}
				ql := strings.ToLower(q)
				for _, sub := range subs {
					if strings.Contains(ql, sub) {
						filtered = append(filtered, q)
						seen[i] = true
						break
					}
				}
			}
		}
		if len(filtered) > 0 {
			queries = filtered
			fmt.Printf("[filter] ABTEST_ONLY active -> running %d selected queries\n", len(queries))
		}
	}

	// Services
	gem := svc.NewGeminiService()
	uib, _ := svc.NewUIBEventService()

	// Timeout per query (seconds)
	timeoutSec := 40
	if s := strings.TrimSpace(os.Getenv("ABTEST_TIMEOUT_SEC")); s != "" {
		if v, e := fmt.Sscanf(s, "%d", &timeoutSec); v == 0 || e != nil {
			timeoutSec = 40
		}
	}

	// Optional sleep between calls (ms) to reduce rate limit risk
	sleepMs := 600
	if sm := strings.TrimSpace(os.Getenv("ABTEST_SLEEP_MS")); sm != "" {
		if v, e := strconv.Atoi(sm); e == nil && v >= 0 {
			sleepMs = v
		}
	}

	started := time.Now()
	// Reproducibility seed & run id
	seed := started.UnixNano()
	r := rand.New(rand.NewSource(seed))
	runID := fmt.Sprintf("abrun-%s-%06d", started.Format("20060102-150405"), r.Intn(1000000))

	// Prompt log path (JSONL). Can override via ABTEST_PROMPT_LOG_FILE
	promptLogDir := filepath.Join("cmd", "abtest", "results", "prompt_logs")
	_ = ensureDir(promptLogDir)
	promptLogPath := strings.TrimSpace(os.Getenv("ABTEST_PROMPT_LOG_FILE"))
	if promptLogPath == "" {
		promptLogPath = filepath.Join(promptLogDir, fmt.Sprintf("promptlog-%s.jsonl", started.Format("20060102-150405")))
	}
	logFull := strings.TrimSpace(os.Getenv("ABTEST_LOG_FULL"))

	results := make([]ResultItem, 0, len(queries)*2)

	for _, q := range queries {
		// Baseline with simple quota-aware retry
		rb := runModeOnce(gem, uib, q, "baseline", timeoutSec, runID, promptLogPath, logFull)
		if isQuotaError(rb.Error) {
			delay := parseRetryDelay(rb.Error)
			fmt.Printf("   ↪ quota hit; sleeping %ds then retry baseline...\n", delay)
			time.Sleep(time.Duration(delay) * time.Second)
			rb = runModeOnce(gem, uib, q, "baseline", timeoutSec, runID, promptLogPath, logFull)
		}
		results = append(results, rb)
		fmt.Printf("[baseline] %s -> %dms error=%v\n", truncate(q, 64), rb.DurationMs, rb.Error != "")
		time.Sleep(time.Duration(sleepMs) * time.Millisecond)

		// Engineered with simple quota-aware retry
		re := runModeOnce(gem, uib, q, "engineered", timeoutSec, runID, promptLogPath, logFull)
		if isQuotaError(re.Error) {
			delay := parseRetryDelay(re.Error)
			fmt.Printf("   ↪ quota hit; sleeping %ds then retry engineered...\n", delay)
			time.Sleep(time.Duration(delay) * time.Second)
			re = runModeOnce(gem, uib, q, "engineered", timeoutSec, runID, promptLogPath, logFull)
		}
		results = append(results, re)
		fmt.Printf("[engineered] %s -> %dms error=%v\n", truncate(q, 64), re.DurationMs, re.Error != "")
		time.Sleep(time.Duration(sleepMs) * time.Millisecond)
	}

	outDir := "cmd/abtest/results"
	if err := ensureDir(outDir); err != nil {
		fmt.Println("failed to create results dir:", err)
		os.Exit(1)
	}
	stamp := time.Now().Format("20060102-150405")
	jsonPath := filepath.Join(outDir, fmt.Sprintf("abtest-%s.json", stamp))
	csvPath := filepath.Join(outDir, fmt.Sprintf("abtest-%s.csv", stamp))

	summary := RunSummary{
		RunID:        runID,
		RandomSeed:   seed,
		StartedAt:    started.Format(time.RFC3339),
		EndedAt:      time.Now().Format(time.RFC3339),
		Env:          config.AppEnv,
		GeminiOn:     config.IsGeminiEnabled,
		Model:        config.GeminiModel,
		Temperature:  0.4,
		ABTestOnly:   strings.TrimSpace(os.Getenv("ABTEST_ONLY")),
		PromptLog:    promptLogPath,
		TotalQueries: len(queries),
		Results:      results,
	}
	if err := writeJSON(jsonPath, summary); err != nil {
		fmt.Println("failed to write JSON:", err)
		os.Exit(1)
	}
	if err := writeCSV(csvPath, results); err != nil {
		fmt.Println("failed to write CSV:", err)
		os.Exit(1)
	}

	fmt.Println("\nSaved:")
	fmt.Println(" -", jsonPath)
	fmt.Println(" -", csvPath)
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-3] + "..."
}

func runModeOnce(gem *svc.GeminiService, uib *svc.UIBEventService, q, mode string, timeoutSec int, runID, promptLogPath, logFull string) ResultItem {
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeoutSec)*time.Second)
	defer cancel()
	// attach abtest metadata for prompt logging
	ctx = context.WithValue(ctx, ctxRunID, runID)
	ctx = context.WithValue(ctx, ctxMode, mode)
	if strings.TrimSpace(promptLogPath) != "" {
		ctx = context.WithValue(ctx, ctxLogFile, promptLogPath)
	}
	if strings.TrimSpace(logFull) != "" {
		ctx = context.WithValue(ctx, ctxLogFull, logFull)
	}
	t0 := time.Now()
	var resp string
	var err error
	switch mode {
	case "baseline":
		resp, err = gem.AskCampus(ctx, q)
	default:
		chat := []svc.ChatMessage{{Role: "user", Text: q}}
		resp, err = gem.AskCampusWithChat(ctx, chat)
	}
	dur := time.Since(t0)
	// derive prompt template id/version and context hash for results
	var tmplID string
	if uib != nil && uib.AnalyzeQueryForUIB(q) {
		if mode == "baseline" {
			tmplID = "askcampus_uib_v1"
		} else {
			tmplID = "askcampus_chat_uib_v1"
		}
	} else {
		if mode == "baseline" {
			tmplID = "askcampus_generic_v1"
		} else {
			tmplID = "askcampus_chat_generic_v1"
		}
	}
	tmplVer := "2025.10.26"
	var ctxHash string
	var ctxSnap string
	var relIDs []string
	if uib != nil && uib.AnalyzeQueryForUIB(q) {
		evs := uib.GetRelevantEventsForQuery(q)
		relIDs = make([]string, 0, len(evs))
		for _, ev := range evs {
			relIDs = append(relIDs, ev.ID)
		}
		ctxStr := uib.FormatEventsForGemini(evs)
		h := sha256.Sum256([]byte(ctxStr))
		ctxHash = hex.EncodeToString(h[:])
		if strings.EqualFold(strings.TrimSpace(os.Getenv("ABTEST_INCLUDE_CONTEXT_SNAPSHOT")), "1") {
			ctxSnap = ctxStr
		}
	}
	r := ResultItem{
		Query:                 q,
		Mode:                  mode,
		Response:              strings.TrimSpace(resp),
		DurationMs:            dur.Milliseconds(),
		Model:                 config.GeminiModel,
		Timestamp:             time.Now().Format(time.RFC3339),
		PromptTemplateID:      tmplID,
		PromptTemplateVersion: tmplVer,
		ContextHash:           ctxHash,
		ContextSnapshot:       ctxSnap,
		RelevantEventIDs:      relIDs,
	}
	if err != nil {
		r.Error = err.Error()
	}
	return r
}

func isQuotaError(errStr string) bool {
	if errStr == "" {
		return false
	}
	s := strings.ToLower(errStr)
	return strings.Contains(s, "resource_exhausted") || strings.Contains(s, "quota exceeded") || strings.Contains(s, " 429")
}

func parseRetryDelay(errStr string) int {
	// Try to extract seconds from a pattern like '"retryDelay": "43s"'
	idx := strings.Index(errStr, "retryDelay")
	if idx >= 0 {
		sub := errStr[idx:]
		// find first number
		num := ""
		for i := 0; i < len(sub); i++ {
			if sub[i] >= '0' && sub[i] <= '9' {
				j := i
				for j < len(sub) && sub[j] >= '0' && sub[j] <= '9' {
					j++
				}
				num = sub[i:j]
				break
			}
		}
		if num != "" {
			if v, e := strconv.Atoi(num); e == nil {
				if v > 0 {
					return v
				}
			}
		}
	}
	return 45
}
