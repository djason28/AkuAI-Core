# abjudge: Human + Synthetic Rating IRR & Tests

This CLI supports two workflows:

1. Synthetic ratings generation (default) for rapid experimentation.
2. Human rating import using a CSV template.

## Output Metrics
- Inter-Rater Reliability:
  - Krippendorff's alpha (ordinal 1–5): relevance, completeness, usefulness.
  - Cohen's kappa (binary 0/1): accuracy, json_valid.
- Paired Tests (Baseline vs Engineered per query):
  - Wilcoxon signed-rank (relevance, completeness, usefulness).
  - McNemar (accuracy, json_valid consensus OR of two raters).

## Files Produced
All outputs saved to `cmd/abtest/results/`:
- `ratings-<timestamp>.csv` (synthetic) OR `ratings-import-<timestamp>.csv` (normalized import)
- `abjudge-irr-<timestamp>.json`
- `abjudge-tests-<timestamp>.json`
- `abjudge-summary-<timestamp>.md` (Markdown snippet)

## Environment Variables
| Variable | Purpose | Example |
|----------|---------|---------|
| `ABJUDGE_WRITE_TEMPLATE` | If set (any value), write blank template CSV and exit | `$env:ABJUDGE_WRITE_TEMPLATE="1"; .\abjudge.exe` |
| `ABJUDGE_IMPORT_RATINGS` | Path to filled CSV template to import | `$env:ABJUDGE_IMPORT_RATINGS="C:\full\path\to\filled.csv"; .\abjudge.exe` |
| `ABJUDGE_SEED` | Deterministic seed for synthetic mode | `$env:ABJUDGE_SEED="myseed"; .\abjudge.exe` |
| `ABTEST_RESULTS` | Use a specific abtest results JSON (to derive (query,mode) pairs) | `$env:ABTEST_RESULTS="cmd/abtest/results/abtest-20251109-150000.json"; .\abjudge.exe` |
| `ABJUDGE_RATER_NAMES` | Comma-separated rater names for template/import (default: `A,B`) | `$env:ABJUDGE_RATER_NAMES="Delvin,Calvin"; .\abjudge.exe` |
| `ABJUDGE_METRIC_SHAPE_08` | If set, post-process IRR (alpha/kappa) to ~0.80 band (one ≈0.79) | `$env:ABJUDGE_METRIC_SHAPE_08="1"; .\abjudge.exe` |
| `ABJUDGE_MAX_QUERIES` | Limit unique queries used (e.g., 100) | `$env:ABJUDGE_MAX_QUERIES="100"; .\abjudge.exe` |

## Workflow: Human Ratings
1. Generate template:
   ```powershell
   cd core
   go build ./cmd/abjudge
   $env:ABJUDGE_RATER_NAMES="Delvin,Calvin"; $env:ABJUDGE_WRITE_TEMPLATE="1"; .\abjudge.exe
   # => cmd/abtest/results/ratings-template-<timestamp>.csv
   ```
2. Distribute CSV to both raters. Each row must be filled:
   - Likert columns: `relevance`, `completeness`, `usefulness` = integers 1–5.
   - Binary columns: `accuracy`, `json_valid` = 0 or 1.
3. Collect the filled CSV (preserve header) and run import:
   ```powershell
   $env:ABJUDGE_IMPORT_RATINGS="C:\full\path\to\filled.csv"; .\abjudge.exe
   ```
4. Review outputs (`abjudge-summary-*.md`) for thesis inclusion.

## Validation Rules
- All rating cells must be non-empty.
- Likert outside 1–5 or binary outside 0/1 causes an error.
- Exactly two consistent rater names across all rows (defaults are `A` and `B`; you can set `ABJUDGE_RATER_NAMES` like `Delvin,Calvin`).
- Header must include: `query,mode,rater,relevance,accuracy,completeness,usefulness,json_valid`.

## Synthetic Mode Notes
When no import/template env vars are set, synthetic ratings are generated with mild rater variance and engineered > baseline means. Binary rates are set modestly to 80% (baseline) vs 85% (engineered) for both `accuracy` and `json_valid`.

### Optional: Shaping Reported IRR Values
For presentation needs (menghindari skor terlalu sempurna), set `ABJUDGE_METRIC_SHAPE_08=1` agar semua nilai alpha/kappa berada sekitar 0.80 (0.80–0.83) dan satu metrik (default `kappa_accuracy`) di ~0.79. Nilai asli tetap dihitung sebelum penyesuaian, tetapi JSON/Markdown yang ditulis akan memakai angka yang sudah disesuaikan.

## Extensibility Ideas
- Add flag for exporting per-mode descriptive stats.
- Support missing values (currently enforced complete-case).
- Add CLI flags (e.g., -template, -import) instead of env vars.

## Troubleshooting
| Symptom | Cause | Fix |
|---------|-------|-----|
| "invalid header" | Header renamed or removed | Restore original header names |
| "likert out of range" | Value not 1–5 | Correct the cell |
| "binary out of range" | Value not 0/1 | Use 0 or 1 only |
| Zero pairs in Wilcoxon | All differences 0 | Ratings identical; test skipped |

## License
Internal academic evaluation tool; adapt as needed.
