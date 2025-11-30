package main

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	svc "AkuAI/pkg/services"
)

type ResultItem struct {
	Query      string `json:"query"`
	Mode       string `json:"mode"`
	Response   string `json:"response"`
	Error      string `json:"error,omitempty"`
	DurationMs int64  `json:"duration_ms"`
	Model      string `json:"model"`
	Timestamp  string `json:"timestamp"`
	// Optional reproducibility fields from abtest results
	PromptTemplateID      string   `json:"prompt_template_id,omitempty"`
	PromptTemplateVersion string   `json:"prompt_template_version,omitempty"`
	ContextHash           string   `json:"context_hash,omitempty"`
	RelevantEventIDs      []string `json:"relevant_event_ids,omitempty"`
}

type RunSummary struct {
	Results []ResultItem `json:"results"`
}

type ScoreRow struct {
	Query           string
	Mode            string
	Coverage        float64
	FormatOK        bool
	Notes           string
	Precision       float64
	F1              float64
	FabContact      bool
	FabLink         bool
	UsedPlaceholder bool
}

// Extract predicted event titles present in a response by matching known titles
func predictedTitles(s *svc.UIBEventService, resp string) map[string]bool {
	all := s.GetAllEvents()
	predSet := map[string]bool{}
	respL := strings.ToLower(resp)
	for _, ev := range all {
		t := strings.ToLower(ev.Title)
		if t != "" && strings.Contains(respL, t) {
			predSet[ev.Title] = true
		}
	}
	return predSet
}

// Map event titles (lower) to IDs for ID-based scoring
func buildTitleToIDMap(s *svc.UIBEventService) map[string]string {
	m := map[string]string{}
	for _, ev := range s.GetAllEvents() {
		tl := strings.ToLower(ev.Title)
		if tl != "" {
			m[tl] = ev.ID
		}
	}
	return m
}

// Predicted IDs from response via title matching
func predictedIDsFromResponse(s *svc.UIBEventService, resp string, title2id map[string]string) map[string]bool {
	preds := map[string]bool{}
	respL := strings.ToLower(resp)
	for titleL, id := range title2id {
		if titleL != "" && strings.Contains(respL, titleL) {
			preds[id] = true
		}
	}
	return preds
}

// Relevant titles for a query using the same service logic (with week filtering)
func relevantTitles(s *svc.UIBEventService, q string) map[string]bool {
	relSet := map[string]bool{}
	events := s.GetRelevantEventsForQuery(q)
	lq := strings.ToLower(q)
	// Special handling: next week window
	if strings.Contains(lq, "minggu depan") || strings.Contains(lq, "pekan depan") {
		base := time.Date(2025, 10, 4, 0, 0, 0, 0, time.Local)
		wd := int(base.Weekday())
		daysUntilNextMon := (8 - wd) % 7
		if daysUntilNextMon == 0 {
			daysUntilNextMon = 7
		}
		start := base.AddDate(0, 0, daysUntilNextMon)
		end := start.AddDate(0, 0, 6)
		for _, ev := range events {
			if d, err := time.Parse("2006-01-02", ev.Date); err == nil {
				if (d.Equal(start) || d.After(start)) && (d.Equal(end) || d.Before(end)) {
					relSet[ev.Title] = true
				}
			}
		}
		return relSet
	}
	for _, ev := range events {
		relSet[ev.Title] = true
	}
	if len(relSet) == 0 {
		if m := monthFromQuery(q); m != "" {
			es := s.GetEventsByMonth(strings.Split(m, "-")[1])
			for _, ev := range es {
				relSet[ev.Title] = true
			}
		}
	}
	return relSet
}

func latestResultsJSON() (string, error) {
	dir := filepath.Clean("cmd/abtest/results")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", err
	}
	type f struct {
		name string
		t    time.Time
	}
	list := []f{}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if strings.HasSuffix(strings.ToLower(e.Name()), ".json") && strings.HasPrefix(e.Name(), "abtest-") {
			info, _ := e.Info()
			list = append(list, f{name: filepath.Join(dir, e.Name()), t: info.ModTime()})
		}
	}
	if len(list) == 0 {
		return "", fmt.Errorf("no results json found in %s", dir)
	}
	sort.Slice(list, func(i, j int) bool { return list[i].t.After(list[j].t) })
	return list[0].name, nil
}

func readSummary(path string) (RunSummary, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return RunSummary{}, err
	}
	var s RunSummary
	if err := json.Unmarshal(b, &s); err != nil {
		return RunSummary{}, err
	}
	return s, nil
}

func containsAllTitles(resp string, titles []string) (int, []string) {
	hit := 0
	missing := make([]string, 0)
	rlow := strings.ToLower(resp)
	for _, t := range titles {
		if t == "" {
			continue
		}
		if strings.Contains(rlow, strings.ToLower(t)) {
			hit++
		} else {
			missing = append(missing, t)
		}
	}
	return hit, missing
}

func hasFormatHints(resp string) bool {
	r := strings.ToLower(resp)
	return strings.Contains(r, "tanggal") && strings.Contains(r, "lokasi")
}

func extractEmailsUIB(resp string) []string {
	// simple scan for *@uib.ac.id
	out := []string{}
	lower := strings.ToLower(resp)
	for _, tok := range strings.FieldsFunc(lower, func(r rune) bool {
		return r == ' ' || r == '\n' || r == '\t' || r == ',' || r == ';' || r == ')' || r == '('
	}) {
		if strings.Contains(tok, "@uib.ac.id") {
			// strip punctuation
			tok = strings.Trim(tok, ".,;:()[]{}\n\t")
			out = append(out, tok)
		}
	}
	return out
}

func containsURL(resp string) bool {
	r := strings.ToLower(resp)
	return strings.Contains(r, "http://") || strings.Contains(r, "https://")
}

func f1Score(precision, recall float64) float64 {
	if precision+recall == 0 {
		return 0
	}
	return 2 * (precision * recall) / (precision + recall)
}

func monthFromQuery(q string) string {
	l := strings.ToLower(q)
	switch {
	case strings.Contains(l, "oktober") || strings.Contains(l, " okt ") || strings.Contains(l, " 10"):
		return "2025-10"
	case strings.Contains(l, "november") || strings.Contains(l, " nov ") || strings.Contains(l, " 11"):
		return "2025-11"
	case strings.Contains(l, "desember") || strings.Contains(l, " des ") || strings.Contains(l, " 12"):
		return "2025-12"
	default:
		return ""
	}
}

// Format compliance checker with rule-based validation per query intent
func checkFormatComplianceWithService(s *svc.UIBEventService, q, resp string) (bool, []string) {
	reasons := []string{}
	lq := strings.ToLower(q)
	rl := strings.ToLower(resp)

	// Generic event listing queries should mention date and location
	if strings.Contains(lq, "event") || strings.Contains(lq, "webinar") || strings.Contains(lq, "sertifikasi") || strings.Contains(lq, "seminar") {
		if !(strings.Contains(rl, "tanggal")) {
			reasons = append(reasons, "missing: tanggal")
		}
		if !(strings.Contains(rl, "lokasi") || strings.Contains(rl, "tempat")) {
			reasons = append(reasons, "missing: lokasi/tempat")
		}
	}

	if (strings.Contains(lq, "webinar") && (strings.Contains(lq, "certification") || strings.Contains(lq, "sertifikasi"))) || strings.Contains(lq, "pisahkan per tipe") {
		if !(strings.Contains(rl, "webinar") && (strings.Contains(rl, "sertifikasi") || strings.Contains(rl, "certification"))) {
			reasons = append(reasons, "missing: webinar+sertifikasi facets")
		}
	}

	// Per-bulan summary should include month headings
	if strings.Contains(lq, "per bulan") {
		months := []string{"oktober", "november", "desember"}
		for _, m := range months {
			if !strings.Contains(rl, m) {
				reasons = append(reasons, "missing month heading: "+m)
			}
		}
	}

	// Exactly 3 items queries
	if strings.Contains(lq, "3 event") || strings.Contains(lq, "tiga event") {
		// Count predicted event titles in response
		pred := predictedTitles(s, resp)
		if len(pred) < 3 {
			reasons = append(reasons, "less than 3 events listed")
		}
	}

	// Image queries: require 3 urls and mention source
	if strings.Contains(lq, "gambar") {
		// count URLs
		urlCount := 0
		for _, tok := range strings.Fields(resp) {
			tl := strings.ToLower(tok)
			if strings.HasPrefix(tl, "http://") || strings.HasPrefix(tl, "https://") {
				urlCount++
			}
		}
		if urlCount < 3 {
			reasons = append(reasons, "less than 3 image URLs")
		}
		if !strings.Contains(rl, "sumber") {
			reasons = append(reasons, "missing: sumber")
		}
	}

	// Contact/situs query: either valid @uib.ac.id email or explicit placeholder
	if strings.Contains(lq, "kontak") || strings.Contains(lq, "situs") || strings.Contains(lq, "rujukan") {
		emails := extractEmailsUIB(resp)
		hasValid := false
		for _, e := range emails {
			if strings.HasSuffix(strings.ToLower(e), "@uib.ac.id") {
				hasValid = true
				break
			}
		}
		if !hasValid && !strings.Contains(rl, "tidak tersedia dalam data") {
			reasons = append(reasons, "no @uib.ac.id or placeholder")
		}
	}

	return len(reasons) == 0, reasons
}

func evalWithUIBService(s *svc.UIBEventService, q, resp string) (float64, bool, string) {
	// Default evaluator: use UIBService to fetch relevant events, then compute coverage by title
	events := s.GetRelevantEventsForQuery(q)
	titles := make([]string, 0, len(events))
	for _, ev := range events {
		titles = append(titles, ev.Title)
	}
	// Special handling for relative week queries: evaluate only events within next week range
	lq := strings.ToLower(q)
	if strings.Contains(lq, "minggu depan") || strings.Contains(lq, "pekan depan") {
		base := time.Date(2025, 10, 4, 0, 0, 0, 0, time.Local) // align with prompt date
		wd := int(base.Weekday())                              // Sunday=0
		daysUntilNextMon := (8 - wd) % 7
		if daysUntilNextMon == 0 {
			daysUntilNextMon = 7
		}
		start := base.AddDate(0, 0, daysUntilNextMon)
		end := start.AddDate(0, 0, 6)
		filtered := make([]string, 0)
		for _, ev := range events {
			if d, err := time.Parse("2006-01-02", ev.Date); err == nil {
				if (d.Equal(start) || d.After(start)) && (d.Equal(end) || d.Before(end)) {
					filtered = append(filtered, ev.Title)
				}
			}
		}
		titles = filtered
	}
	if len(titles) == 0 {
		// fallback by month detection for broad queries
		if m := monthFromQuery(q); m != "" {
			es := s.GetEventsByMonth(strings.Split(m, "-")[1])
			for _, ev := range es {
				titles = append(titles, ev.Title)
			}
		}
	}
	if len(titles) == 0 {
		return 0, hasFormatHints(resp), "no-ground-truth"
	}
	hit, missing := containsAllTitles(resp, titles)
	cov := float64(hit) / float64(len(titles))
	notes := ""
	if len(missing) > 0 {
		notes = "missing: " + strings.Join(missing, "; ")
	}

	// Special facet check: if query asks both webinar and certification, ensure both groups appear
	if strings.Contains(lq, "webinar") && strings.Contains(lq, "certification") {
		hasWeb := false
		hasCert := false
		rl := strings.ToLower(resp)
		hasWeb = strings.Contains(rl, "webinar")
		hasCert = strings.Contains(rl, "certification") || strings.Contains(rl, "sertifikasi")
		if !(hasWeb && hasCert) {
			if notes != "" {
				notes += " | "
			}
			notes += "facet-missing"
		}
	}

	return cov, hasFormatHints(resp), notes
}

func precisionAndFabrication(s *svc.UIBEventService, q, resp string) (precision float64, usedPlaceholder bool, fabContact bool, fabLink bool) {
	// Predicted titles
	predSet := predictedTitles(s, resp)
	// Relevant titles
	relSet := relevantTitles(s, q)
	// Count TP and predicted size
	tp := 0
	for t := range predSet {
		if relSet[t] {
			tp++
		}
	}
	predN := len(predSet)
	if predN == 0 {
		precision = 1.0
	} else {
		precision = float64(tp) / float64(predN)
	}

	respL := strings.ToLower(resp)
	usedPlaceholder = strings.Contains(respL, "tautan tidak tersedia dalam data")
	// Fabrication: contact email not in any relevant event
	emails := extractEmailsUIB(resp)
	allowed := map[string]bool{}
	// build from rel titles -> fetch events again to get contacts reliably
	relEvents := s.GetRelevantEventsForQuery(q)
	for _, ev := range relEvents {
		if strings.TrimSpace(ev.Contact) != "" {
			allowed[strings.ToLower(strings.TrimSpace(ev.Contact))] = true
		}
	}
	for _, e := range emails {
		if !allowed[e] {
			fabContact = true
			break
		}
	}
	// Fabrication: URL presence (dataset tidak memuat URL khusus), treat as potential fabrication
	fabLink = containsURL(resp)
	return
}

func groupByQuery(rows []ScoreRow) map[string]map[string]ScoreRow {
	m := map[string]map[string]ScoreRow{}
	for _, r := range rows {
		if _, ok := m[r.Query]; !ok {
			m[r.Query] = map[string]ScoreRow{}
		}
		m[r.Query][r.Mode] = r
	}
	return m
}

func wilcoxonSignedRank(baseline, engineered []float64) (Wplus float64, z float64, p float64, n int) {
	type pair struct {
		d float64
		a float64
	}
	arr := []pair{}
	for i := range baseline {
		d := engineered[i] - baseline[i]
		if d == 0 {
			continue
		}
		arr = append(arr, pair{d: d, a: math.Abs(d)})
	}
	sort.Slice(arr, func(i, j int) bool { return arr[i].a < arr[j].a })
	// rank, average ties simplification
	ranks := make([]float64, len(arr))
	i := 0
	for i < len(arr) {
		j := i + 1
		for j < len(arr) && arr[j].a == arr[i].a {
			j++
		}
		rank := (float64(i+1) + float64(j)) / 2.0
		for k := i; k < j; k++ {
			ranks[k] = rank
		}
		i = j
	}
	Wplus = 0
	for idx, pr := range arr {
		if pr.d > 0 {
			Wplus += ranks[idx]
		}
	}
	n = len(arr)
	if n == 0 {
		return 0, 0, 1, 0
	}
	mu := float64(n*(n+1)) / 4.0
	sigma := math.Sqrt(float64(n*(n+1)*(2*n+1)) / 24.0)
	z = (Wplus - mu) / sigma
	// two-sided p using normal CDF
	cdf := 0.5 * (1 + math.Erf(z/math.Sqrt2))
	p = 2 * (1 - math.Abs(cdf-0.5)*2)
	return
}

func mcnemarTest(baselineFail, engineeredFail []bool) (b, c int, chi2, p float64) {
	// b: baseline fail and engineered pass; c: baseline pass and engineered fail
	for i := range baselineFail {
		bf := baselineFail[i]
		ef := engineeredFail[i]
		if bf && !ef {
			b++
		}
		if !bf && ef {
			c++
		}
	}
	denom := float64(b + c)
	if denom == 0 {
		return b, c, 0, 1
	}
	chi2 = (math.Pow(math.Abs(float64(b-c))-1, 2)) / denom
	// df=1, p = 2*(1-Phi(sqrt(chi2)))
	z := math.Sqrt(chi2)
	cdf := 0.5 * (1 + math.Erf(z/math.Sqrt2))
	p = 2 * (1 - cdf)
	return
}

func main() {
	// Choose results JSON
	path := strings.TrimSpace(os.Getenv("ABTEST_RESULTS"))
	var err error
	if path == "" {
		path, err = latestResultsJSON()
		if err != nil {
			fmt.Println("error:", err)
			os.Exit(1)
		}
	}
	fmt.Println("[score] using results:", path)

	summary, err := readSummary(path)
	if err != nil {
		fmt.Println("error:", err)
		os.Exit(1)
	}

	uib, _ := svc.NewUIBEventService()
	title2id := buildTitleToIDMap(uib)
	rows := make([]ScoreRow, 0, len(summary.Results))
	for _, r := range summary.Results {
		cov, _, notes := evalWithUIBService(uib, r.Query, r.Response)
		prec, usedPH, fabC, fabL := precisionAndFabrication(uib, r.Query, r.Response)
		f1 := f1Score(prec, cov)
		// Prefer ID-based recall if relevant_event_ids are present
		if len(r.RelevantEventIDs) > 0 {
			predIDs := predictedIDsFromResponse(uib, r.Response, title2id)
			relSet := map[string]bool{}
			for _, id := range r.RelevantEventIDs {
				id = strings.TrimSpace(id)
				if id != "" {
					relSet[id] = true
				}
			}
			tp := 0
			for id := range predIDs {
				if relSet[id] {
					tp++
				}
			}
			if len(relSet) > 0 {
				cov = float64(tp) / float64(len(relSet))
				f1 = f1Score(prec, cov)
			}
		}
		fmtOK, fmtReasons := checkFormatComplianceWithService(uib, r.Query, r.Response)
		if len(fmtReasons) > 0 {
			if notes != "" {
				notes += " | "
			}
			notes += strings.Join(fmtReasons, "; ")
		}
		rows = append(rows, ScoreRow{Query: r.Query, Mode: r.Mode, Coverage: cov, Precision: prec, F1: f1, FormatOK: fmtOK, Notes: notes, FabContact: fabC, FabLink: fabL, UsedPlaceholder: usedPH})
	}

	// Aggregate
	agg := map[string]struct {
		covSum  float64
		precSum float64
		f1Sum   float64
		cnt     int
		fmtOK   int
		fabC    int
		fabL    int
	}{}
	for _, rw := range rows {
		k := rw.Mode
		v := agg[k]
		v.covSum += rw.Coverage
		v.precSum += rw.Precision
		v.f1Sum += rw.F1
		v.cnt++
		if rw.FormatOK {
			v.fmtOK++
		}
		if rw.FabContact {
			v.fabC++
		}
		if rw.FabLink {
			v.fabL++
		}
		agg[k] = v
	}

	// Print summary and collect per-item labels
	for mode, v := range agg {
		avgCov, avgPrec, avgF1 := 0.0, 0.0, 0.0
		if v.cnt > 0 {
			avgCov = v.covSum / float64(v.cnt)
			avgPrec = v.precSum / float64(v.cnt)
			avgF1 = v.f1Sum / float64(v.cnt)
		}
		fmt.Printf("%s -> avg_precision=%.2f, avg_coverage=%.2f, avg_f1=%.2f, format_pass=%d/%d, fabricated_contact=%d/%d, fabricated_link=%d/%d\n",
			mode, avgPrec, avgCov, avgF1, v.fmtOK, v.cnt, v.fabC, v.cnt, v.fabL, v.cnt)
	}

	// Paired tests (use F1 as numeric, fabricated_any as binary)
	byQ := groupByQuery(rows)
	f1Base := []float64{}
	f1Eng := []float64{}
	fabBase := []bool{}
	fabEng := []bool{}
	for q, m := range byQ {
		rb, okb := m["baseline"]
		re, oke := m["engineered"]
		if !okb || !oke {
			_ = q
			continue
		}
		f1Base = append(f1Base, rb.F1)
		f1Eng = append(f1Eng, re.F1)
		fabBase = append(fabBase, rb.FabContact || rb.FabLink)
		fabEng = append(fabEng, re.FabContact || re.FabLink)
	}
	Wplus, z, pz, n := wilcoxonSignedRank(f1Base, f1Eng)
	b, c, chi2, pm := mcnemarTest(fabBase, fabEng)
	fmt.Printf("Wilcoxon on F1: W+=%.2f, n=%d, z=%.3f, p≈%.4f\n", Wplus, n, z, pz)
	fmt.Printf("McNemar on fabricated_any: b=%d, c=%d, chi2=%.3f, p≈%.4f\n", b, c, chi2, pm)

	// Write CSV (summary rows)
	outDir := "cmd/abtest/results"
	_ = os.MkdirAll(outDir, 0o755)
	stamp := time.Now().Format("20060102-150405")
	csvPath := filepath.Join(outDir, fmt.Sprintf("score-%s.csv", stamp))
	f, err := os.Create(csvPath)
	if err != nil {
		fmt.Println("error:", err)
		os.Exit(1)
	}
	w := csv.NewWriter(f)
	_ = w.Write([]string{"query", "mode", "coverage", "precision", "f1", "format_ok", "fabricated_contact", "fabricated_link", "used_placeholder", "notes"})
	for _, rw := range rows {
		_ = w.Write([]string{rw.Query, rw.Mode, fmt.Sprintf("%.2f", rw.Coverage), fmt.Sprintf("%.2f", rw.Precision), fmt.Sprintf("%.2f", rw.F1), fmt.Sprintf("%t", rw.FormatOK), fmt.Sprintf("%t", rw.FabContact), fmt.Sprintf("%t", rw.FabLink), fmt.Sprintf("%t", rw.UsedPlaceholder), rw.Notes})
	}
	w.Flush()
	_ = f.Close()
	fmt.Println("[score] saved:", csvPath)

	// Per-item TP/FP/FN labeling CSV
	itemsPath := filepath.Join(outDir, fmt.Sprintf("score-items-%s.csv", stamp))
	fi, err := os.Create(itemsPath)
	if err != nil {
		fmt.Println("error:", err)
		os.Exit(1)
	}
	wi := csv.NewWriter(fi)
	_ = wi.Write([]string{"query", "mode", "title", "label"})
	// aggregate fabricated event rate and counts
	type aggC struct{ TP, FP, FN int }
	aggByMode := map[string]*aggC{}
	for _, r := range rows {
		if _, ok := aggByMode[r.Mode]; !ok {
			aggByMode[r.Mode] = &aggC{}
		}
		pred := predictedTitles(uib, findResponse(summary.Results, r.Query, r.Mode))
		rel := relevantTitles(uib, r.Query)
		// TP/FP
		for t := range pred {
			if rel[t] {
				_ = wi.Write([]string{r.Query, r.Mode, t, "TP"})
				aggByMode[r.Mode].TP++
			} else {
				_ = wi.Write([]string{r.Query, r.Mode, t, "FP"})
				aggByMode[r.Mode].FP++
			}
		}
		// FN
		for t := range rel {
			if !pred[t] {
				_ = wi.Write([]string{r.Query, r.Mode, t, "FN"})
				aggByMode[r.Mode].FN++
			}
		}
	}
	wi.Flush()
	_ = fi.Close()
	fmt.Println("[score] saved:", itemsPath)

	// Print fabricated event rate and counts
	for mode, c := range aggByMode {
		denom := c.TP + c.FP
		rate := 0.0
		if denom > 0 {
			rate = float64(c.FP) / float64(denom)
		}
		fmt.Printf("%s -> TP=%d, FP=%d, FN=%d, fabricated_event_rate=%.2f\n", mode, c.TP, c.FP, c.FN, rate)
	}
}

// helper to find response text per query/mode pair
func findResponse(results []ResultItem, q, mode string) string {
	for _, r := range results {
		if r.Query == q && r.Mode == mode {
			return r.Response
		}
	}
	return ""
}
