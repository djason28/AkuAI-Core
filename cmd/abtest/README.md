# A/B Prompt Test Runner

This CLI runs 20 queries through two prompt strategies using the existing GeminiService:
- baseline: `AskCampus` (no explicit UIB context injection)
- engineered: `AskCampusWithChat` (detects UIB queries and injects curated UIB event context)

It saves JSON and CSV results with timestamps for analysis in Bab 4.6.

See the latest write-up in:
- `core/docs/bab-4.6-evaluasi-abtest.md` (methods, metrics, significance, and reproducibility)

## Prerequisites
- Go 1.24+
- Configure backend environment variables in `core/.env`:
  - `IS_GEMINI_ENABLED=1`
  - `GEMINI_API_KEY=...`
  - optional: `GEMINI_MODEL=...`

Note: The runner calls Gemini directly via the service layer and does not use HTTP or cache.

## Files
- `queries.json`: the 20 test queries
- `results/abtest-YYYYmmdd-HHMMSS.json`: full results with metadata
- `results/abtest-YYYYmmdd-HHMMSS.csv`: flat summary suitable for scoring in spreadsheet

## Run (Windows PowerShell)

Important: The Go module (go.mod) is inside `core/`. Run the command from the `core` folder, and set `APP_ENV=staging` to allow loading `.env` locally.

```powershell
# From repo root
cd .\core

# Ensure .env is loaded (config loads .env when APP_ENV != production)
$env:APP_ENV="staging"; $env:ABTEST_TIMEOUT_SEC="40"; go run ./cmd/abtest
```

If you see warnings about disabled Gemini or empty API key, set `.env` properly and rerun.

## Output Schema
- JSON: includes env, model, and an array of results `{query, mode, response, error, duration_ms, timestamp}`
- CSV: columns `query,mode,duration_ms,model,error,response`

## Scoring (Rubric)
Score each pair (baseline vs engineered) using the Bab 3/4 rubric:
- Relevansi (1–5)
- Akurasi faktual (0/1)
- Kelengkapan (1–5)
- Kegunaan (1–5)

Recommended: create a spreadsheet with conditional formatting; compute deltas between modes.

Note: The scorer now prefers ID-based recall when `relevant_event_ids` are present in results for stricter evaluation.

## Notes
- Image queries (18–19) assess the separate image pipeline and fallback; include the textual reasoning but test images via the `/api/images` endpoints if needed.
- The runner does not persist any PII. Keep raw outputs and your manual scores in versioned folders for reproducibility.

### Baseline-Real in Staging (for local A/B)
By default, baseline uses mock in staging. To force real baseline calls without switching to production:

```powershell
$env:APP_ENV="staging"; $env:ABTEST_FORCE_REAL="1"; $env:ABTEST_TIMEOUT_SEC="40"; go run ./cmd/abtest
```

### Throttling & Retry
- You can add a small delay between calls:

```powershell
$env:APP_ENV="staging"; $env:ABTEST_FORCE_REAL="1"; $env:ABTEST_SLEEP_MS="800"; go run ./cmd/abtest
```

- The runner will attempt a single retry when it hits 429 and will respect the `retryDelay` hint if present.

### Optional: Build a binary
```powershell
cd .\core
$env:APP_ENV="staging"; go build -o .\bin\abtest.exe .\cmd\abtest
 .\bin\abtest.exe
```
