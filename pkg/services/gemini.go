package services

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"regexp"
	"strings"
	"time"

	"AkuAI/pkg/config"
)

type GeminiService struct {
	apiKey     string
	enabled    bool
	uibService *UIBEventService
}

var universityAliasMap = map[string]string{
	"UIB":                                   "Universitas Internasional Batam",
	"UNIVERSITAS INTERNASIONAL BATAM":       "Universitas Internasional Batam",
	"UNIVERSITAS INTERNASIONAL BATAM (UIB)": "Universitas Internasional Batam",
	"UNIVERSITAS INDONESIA":                 "Universitas Indonesia",
	"UNIVERSITAS GADJAH MADA":               "Universitas Gadjah Mada",
	"INSTITUT TEKNOLOGI BANDUNG":            "Institut Teknologi Bandung",
	"ITB":                                   "Institut Teknologi Bandung",
	"IPB":                                   "Institut Pertanian Bogor",
	"UNIVERSITAS AIRLANGGA":                 "Universitas Airlangga",
	"BINUS":                                 "Bina Nusantara University",
	"BINUS UNIVERSITY":                      "Bina Nusantara University",
}

var universityRegex = regexp.MustCompile(`(?i)(universitas|universiti|university|institut|institute|politeknik|sekolah tinggi|college)[^.,;:\n]{0,80}`)

var (
	ErrGeminiDisabled = errors.New("gemini is disabled via config")
)

func NewGeminiService() *GeminiService {
	uibService, err := NewUIBEventService()
	if err != nil {
		log.Printf("[gemini] ❌ CRITICAL: Failed to initialize UIB service: %v", err)
		log.Printf("[gemini] ❌ UIB queries will NOT work properly!")
		// Continue without UIB service - not critical
	} else {
		log.Printf("[gemini] ✅ UIB service initialized successfully")
		allEvents := uibService.GetAllEvents()
		log.Printf("[gemini] ✅ UIB service loaded %d events total", len(allEvents))
		novEvents := uibService.GetEventsByMonth("november")
		log.Printf("[gemini] ✅ UIB service has %d November events", len(novEvents))
	}

	return &GeminiService{
		apiKey:     config.GeminiAPIKey,
		enabled:    config.IsGeminiEnabled,
		uibService: uibService,
	}
}

type ChatMessage struct {
	Role string
	Text string
}

func (s *GeminiService) AskCampus(ctx context.Context, question string) (string, error) {
	// Mock logic: always mock if staging, or if production but disabled
	if config.IsStaging || (config.IsProduction && !config.IsGeminiEnabled) {
		log.Printf("[gemini] MOCK MODE: returning mock chat response")
		return "[MOCK] Halo! Ini adalah jawaban mock dari Gemini. Silakan tanya apa saja tentang UIB.", nil
	}
	if !s.enabled {
		log.Printf("[gemini] disabled via config (IsGeminiEnabled=false)")
		return "", ErrGeminiDisabled
	}
	if strings.TrimSpace(s.apiKey) == "" {
		log.Printf("[gemini] GEMINI_API_KEY is not set")
		return "", fmt.Errorf("GEMINI_API_KEY is not set")
	}

	// Check if question is related to UIB and add context if available
	var prompt string
	log.Printf("[gemini] 🔍 DEBUGGING - Question: %s", question)

	if s.uibService == nil {
		log.Printf("[gemini] ❌ UIB service is nil - UIB features disabled")
	} else {
		isUIBQuery := s.uibService.AnalyzeQueryForUIB(question)
		log.Printf("[gemini] 🔍 UIB detection result: %t", isUIBQuery)
	}

	if s.uibService != nil && s.uibService.AnalyzeQueryForUIB(question) {
		log.Printf("[gemini] ✅ UIB-RELATED QUERY DETECTED! Adding UIB context")

		relevantEvents := s.uibService.GetRelevantEventsForQuery(question)
		log.Printf("[gemini] Found %d relevant UIB events", len(relevantEvents))
		uibContext := s.uibService.FormatEventsForGemini(relevantEvents)

		prompt = fmt.Sprintf(`TANGGAL HARI INI: 4 Oktober 2025

Kamu adalah asisten AI untuk Universitas Internasional Batam (UIB). Jawab pertanyaan menggunakan data resmi UIB yang disediakan di bawah ini.

%s

INSTRUKSI PENTING:
1. PENTING: Hari ini adalah 4 Oktober 2025, jadi semua acara Oktober-Desember 2025 adalah SAAT INI atau AKAN DATANG
2. LANGSUNG berikan SEMUA data yang tersedia sesuai pertanyaan - JANGAN tanya balik atau minta klarifikasi
3. Jika ditanya tentang sertifikasi/webinar per bulan, tampilkan SEMUA yang ada di bulan tersebut
4. SELALU gunakan data UIB yang disediakan di atas sebagai sumber utama
5. Format jawaban dengan struktur jelas: Nama acara, tanggal, waktu, lokasi, biaya, kontak
6. SELALU sebutkan bahwa ini adalah data resmi UIB (UIB_OFFICIAL) 
7. Jika tidak ada data untuk bulan yang ditanyakan, baru katakan tidak tersedia
8. JANGAN katakan "memerlukan informasi lebih lanjut" - langsung berikan semua yang ada
9. Gunakan format: "Berikut sertifikasi UIB untuk [bulan]:" lalu list semua

Pertanyaan: %s`, uibContext, question)
	} else {
		log.Printf("[gemini] ❌ NON-UIB QUERY - Using default prompt")
		prompt = fmt.Sprintf("Jawab secara rinci, terstruktur, dan mudah dipahami tentang informasi kampus. Gunakan Bahasa Indonesia yang jelas. Sertakan poin-poin penting, contoh jika relevan, dan langkah-langkah praktis. Jika ada ketidakpastian, sebutkan asumsi atau saran lanjutan. Pertanyaan: %s", question)
	}

	models := []string{config.GeminiModel, "gemini-2.0-flash"}
	tried := make(map[string]error)

	for _, m := range models {
		if strings.TrimSpace(m) == "" {
			continue
		}
		text, err := s.callGenerateContent(ctx, m, prompt)
		if err != nil && isRetriable(err) {
			sleepWithContext(ctx, 2*time.Second)
			text, err = s.callGenerateContent(ctx, m, prompt)
		}
		if err == nil && strings.TrimSpace(text) != "" {
			return strings.TrimSpace(text), nil
		}
		if err != nil {
			tried[m] = err
			log.Printf("[gemini] model %s failed: %v", m, err)
		}
	}

	var b strings.Builder
	b.WriteString("all gemini models failed: ")
	first := true
	for m, e := range tried {
		if !first {
			b.WriteString("; ")
		}
		first = false
		b.WriteString(fmt.Sprintf("%s -> %v", m, e))
	}
	return "", errors.New(b.String())
}

func (s *GeminiService) AskCampusWithChat(ctx context.Context, chat []ChatMessage) (string, error) {
	if !s.enabled {
		log.Printf("[gemini] disabled via config (IsGeminiEnabled=false)")
		return "", ErrGeminiDisabled
	}
	if strings.TrimSpace(s.apiKey) == "" {
		log.Printf("[gemini] GEMINI_API_KEY is not set")
		return "", fmt.Errorf("GEMINI_API_KEY is not set")
	}

	models := []string{config.GeminiModel, "gemini-2.0-flash"}
	tried := make(map[string]error)

	// Extract the latest user question for UIB context detection
	var latestUserQuestion string
	for i := len(chat) - 1; i >= 0; i-- {
		if strings.ToLower(strings.TrimSpace(chat[i].Role)) == "user" {
			latestUserQuestion = chat[i].Text
			break
		}
	}

	// Check for UIB context and build system instruction
	var systemInstruction string
	if s.uibService != nil && s.uibService.AnalyzeQueryForUIB(latestUserQuestion) {
		log.Printf("[gemini] ✅ UIB-RELATED CHAT QUERY DETECTED! Adding UIB context")
		relevantEvents := s.uibService.GetRelevantEventsForQuery(latestUserQuestion)
		log.Printf("[gemini] Found %d relevant UIB events for chat", len(relevantEvents))
		uibContext := s.uibService.FormatEventsForGemini(relevantEvents)

		systemInstruction = fmt.Sprintf(`TANGGAL HARI INI: 4 Oktober 2025

Kamu adalah asisten AI untuk Universitas Internasional Batam (UIB). Jawab pertanyaan menggunakan data resmi UIB yang disediakan di bawah ini.

%s

INSTRUKSI PENTING:
1. PENTING: Hari ini adalah 4 Oktober 2025, jadi semua acara Oktober-Desember 2025 adalah SAAT INI atau AKAN DATANG
2. LANGSUNG berikan SEMUA data yang tersedia sesuai pertanyaan - JANGAN tanya balik atau minta klarifikasi
3. Jika ditanya tentang sertifikasi/webinar per bulan, tampilkan SEMUA yang ada di bulan tersebut
4. SELALU gunakan data UIB yang disediakan di atas sebagai sumber utama
5. Format jawaban dengan struktur jelas: Nama acara, tanggal, waktu, lokasi, biaya, kontak
6. SELALU sebutkan bahwa ini adalah data resmi UIB (UIB_OFFICIAL) 
7. Jika tidak ada data untuk bulan yang ditanyakan, baru katakan tidak tersedia
8. JANGAN katakan "memerlukan informasi lebih lanjut" - langsung berikan semua yang ada
9. Gunakan format: "Berikut sertifikasi UIB untuk [bulan]:" lalu list semua
10. Jawab dalam Bahasa Indonesia yang jelas dan terstruktur`, uibContext)
	} else {
		log.Printf("[gemini] ❌ NON-UIB CHAT QUERY - Using default system instruction")
		systemInstruction = "Anda adalah asisten kampus yang sangat membantu. Jawab secara rinci, terstruktur (gunakan poin-poin atau langkah), dan jelas dalam Bahasa Indonesia. Jika konteks tidak cukup, minta klarifikasi singkat. Tetap fokus pada topik akademik/kampus."
	}

	payloadBuilder := func() ([]byte, error) {
		contents := make([]any, 0, len(chat))
		for _, m := range chat {
			role := strings.ToLower(strings.TrimSpace(m.Role))
			if role != "user" && role != "model" {
				role = "user"
			}
			contents = append(contents, map[string]any{
				"role":  role,
				"parts": []any{map[string]any{"text": m.Text}},
			})
		}
		reqBody := map[string]any{
			"systemInstruction": map[string]any{
				"parts": []any{map[string]any{"text": systemInstruction}},
			},
			"contents": contents,
			"generationConfig": map[string]any{
				"temperature":     0.6,
				"maxOutputTokens": 2048,
				"topK":            40,
				"topP":            0.9,
			},
		}
		return json.Marshal(reqBody)
	}

	for _, m := range models {
		if strings.TrimSpace(m) == "" {
			continue
		}
		bodyBytes, _ := payloadBuilder()
		text, err := s.callGenerateContentWithBody(ctx, m, bodyBytes)
		if err != nil && isRetriable(err) {
			sleepWithContext(ctx, 2*time.Second)
			text, err = s.callGenerateContentWithBody(ctx, m, bodyBytes)
		}
		if err == nil && strings.TrimSpace(text) != "" {
			return strings.TrimSpace(text), nil
		}
		if err != nil {
			tried[m] = err
			log.Printf("[gemini] model %s failed: %v", m, err)
		}
	}
	var b strings.Builder
	b.WriteString("all gemini models failed: ")
	first := true
	for m, e := range tried {
		if !first {
			b.WriteString("; ")
		}
		first = false
		b.WriteString(fmt.Sprintf("%s -> %v", m, e))
	}
	return "", errors.New(b.String())
}

func (s *GeminiService) StreamCampus(ctx context.Context, question string, onDelta func(string)) (string, error) {
	if !s.enabled {
		log.Printf("[gemini] disabled via config (IsGeminiEnabled=false)")
		return "", ErrGeminiDisabled
	}
	if strings.TrimSpace(s.apiKey) == "" {
		log.Printf("[gemini] GEMINI_API_KEY is not set")
		return "", fmt.Errorf("GEMINI_API_KEY is not set")
	}

	prompt := fmt.Sprintf("Jawab secara rinci, terstruktur, dan mudah dipahami tentang informasi kampus. Gunakan Bahasa Indonesia yang jelas. Sertakan poin-poin penting, contoh jika relevan, dan langkah-langkah praktis. Jika ada ketidakpastian, sebutkan asumsi atau saran lanjutan. Pertanyaan: %s", question)

	models := []string{config.GeminiModel, "gemini-2.0-flash"}
	tried := make(map[string]error)

	for _, m := range models {
		if strings.TrimSpace(m) == "" {
			continue
		}
		text, err := s.callStreamGenerateContent(ctx, m, prompt, onDelta)
		if err != nil && isRetriable(err) {
			sleepWithContext(ctx, 2*time.Second)
			text, err = s.callStreamGenerateContent(ctx, m, prompt, onDelta)
		}
		if err == nil {
			if strings.TrimSpace(text) != "" {
				return strings.TrimSpace(text), nil
			}
			if full, gerr := s.callGenerateContent(ctx, m, prompt); gerr == nil && strings.TrimSpace(full) != "" {
				if onDelta != nil {
					onDelta(full)
				}
				return strings.TrimSpace(full), nil
			}
		}
		if err != nil {
			tried[m] = err
			log.Printf("[gemini] stream model %s failed: %v", m, err)
		}
	}
	var b strings.Builder
	b.WriteString("all gemini stream models failed: ")
	first := true
	for m, e := range tried {
		if !first {
			b.WriteString("; ")
		}
		first = false
		b.WriteString(fmt.Sprintf("%s -> %v", m, e))
	}
	return "", errors.New(b.String())
}

func (s *GeminiService) StreamCampusWithChat(ctx context.Context, chat []ChatMessage, onDelta func(string)) (string, error) {
	if !s.enabled {
		log.Printf("[gemini] disabled via config (IsGeminiEnabled=false)")
		return "", ErrGeminiDisabled
	}
	if strings.TrimSpace(s.apiKey) == "" {
		log.Printf("[gemini] GEMINI_API_KEY is not set")
		return "", fmt.Errorf("GEMINI_API_KEY is not set")
	}

	models := []string{config.GeminiModel, "gemini-2.0-flash"}
	tried := make(map[string]error)

	// Extract the latest user question for UIB context detection
	var latestUserQuestion string
	for i := len(chat) - 1; i >= 0; i-- {
		if strings.ToLower(strings.TrimSpace(chat[i].Role)) == "user" {
			latestUserQuestion = chat[i].Text
			break
		}
	}

	// Check for UIB context and build system instruction
	var systemInstruction string
	if s.uibService != nil && s.uibService.AnalyzeQueryForUIB(latestUserQuestion) {
		log.Printf("[gemini] ✅ UIB-RELATED STREAM QUERY DETECTED! Adding UIB context")
		relevantEvents := s.uibService.GetRelevantEventsForQuery(latestUserQuestion)
		log.Printf("[gemini] Found %d relevant UIB events for streaming", len(relevantEvents))
		uibContext := s.uibService.FormatEventsForGemini(relevantEvents)

		systemInstruction = fmt.Sprintf(`TANGGAL HARI INI: 4 Oktober 2025

Kamu adalah asisten AI untuk Universitas Internasional Batam (UIB). Jawab pertanyaan menggunakan data resmi UIB yang disediakan di bawah ini.

%s

INSTRUKSI PENTING:
1. PENTING: Hari ini adalah 4 Oktober 2025, jadi semua acara Oktober-Desember 2025 adalah SAAT INI atau AKAN DATANG
2. LANGSUNG berikan SEMUA data yang tersedia sesuai pertanyaan - JANGAN tanya balik atau minta klarifikasi
3. Jika ditanya tentang sertifikasi/webinar per bulan, tampilkan SEMUA yang ada di bulan tersebut
4. SELALU gunakan data UIB yang disediakan di atas sebagai sumber utama
5. Format jawaban dengan struktur jelas: Nama acara, tanggal, waktu, lokasi, biaya, kontak
6. SELALU sebutkan bahwa ini adalah data resmi UIB (UIB_OFFICIAL) 
7. Jika tidak ada data untuk bulan yang ditanyakan, baru katakan tidak tersedia
8. JANGAN katakan "memerlukan informasi lebih lanjut" - langsung berikan semua yang ada
9. Gunakan format: "Berikut sertifikasi UIB untuk [bulan]:" lalu list semua
10. Jawab dalam Bahasa Indonesia yang jelas dan terstruktur`, uibContext)
	} else {
		log.Printf("[gemini] ❌ NON-UIB STREAM QUERY - Using default system instruction")
		systemInstruction = "Anda adalah asisten kampus yang sangat membantu. Jawab secara rinci, terstruktur (gunakan poin-poin atau langkah), dan jelas dalam Bahasa Indonesia. Jika konteks tidak cukup, minta klarifikasi singkat. Tetap fokus pada topik akademik/kampus."
	}

	payloadBuilder := func() ([]byte, error) {
		contents := make([]any, 0, len(chat))
		for _, m := range chat {
			role := strings.ToLower(strings.TrimSpace(m.Role))
			if role != "user" && role != "model" {
				role = "user"
			}
			contents = append(contents, map[string]any{
				"role":  role,
				"parts": []any{map[string]any{"text": m.Text}},
			})
		}
		reqBody := map[string]any{
			"systemInstruction": map[string]any{
				"parts": []any{map[string]any{"text": systemInstruction}},
			},
			"contents": contents,
			"generationConfig": map[string]any{
				"temperature":     0.6,
				"maxOutputTokens": 2048,
				"topK":            40,
				"topP":            0.9,
			},
		}
		return json.Marshal(reqBody)
	}

	for _, m := range models {
		if strings.TrimSpace(m) == "" {
			continue
		}
		bodyBytes, _ := payloadBuilder()
		text, err := s.callStreamGenerateContentWithBody(ctx, m, bodyBytes, onDelta)
		if err != nil && isRetriable(err) {
			sleepWithContext(ctx, 2*time.Second)
			text, err = s.callStreamGenerateContentWithBody(ctx, m, bodyBytes, onDelta)
		}
		if err == nil {
			if strings.TrimSpace(text) != "" {
				return strings.TrimSpace(text), nil
			}
			if full, gerr := s.callGenerateContentWithBody(ctx, m, bodyBytes); gerr == nil && strings.TrimSpace(full) != "" {
				if onDelta != nil {
					onDelta(full)
				}
				return strings.TrimSpace(full), nil
			}
		}
		if err != nil {
			tried[m] = err
			log.Printf("[gemini] stream model %s failed: %v", m, err)
		}
	}
	var b strings.Builder
	b.WriteString("all gemini stream models failed: ")
	first := true
	for m, e := range tried {
		if !first {
			b.WriteString("; ")
		}
		first = false
		b.WriteString(fmt.Sprintf("%s -> %v", m, e))
	}
	return "", errors.New(b.String())
}

func (s *GeminiService) callGenerateContent(ctx context.Context, model, prompt string) (string, error) {
	reqBody := map[string]any{
		"contents": []any{
			map[string]any{
				"role":  "user",
				"parts": []any{map[string]any{"text": prompt}},
			},
		},
		"generationConfig": map[string]any{
			"temperature":     0.6,
			"maxOutputTokens": 1024,
			"topK":            40,
			"topP":            0.9,
		},
	}
	bodyBytes, _ := json.Marshal(reqBody)

	url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent?key=%s", model, s.apiKey)
	log.Printf("[gemini] using model %s", model)
	log.Printf("[gemini] POST %s", url)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(bodyBytes))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("http error: %w", err)
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read error: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("status %d: %s", resp.StatusCode, strings.TrimSpace(string(respBytes)))
	}

	var parsed map[string]any
	if err := json.Unmarshal(respBytes, &parsed); err != nil {
		return strings.TrimSpace(string(respBytes)), nil
	}
	if cands, ok := parsed["candidates"].([]any); ok && len(cands) > 0 {
		if first, ok := cands[0].(map[string]any); ok {
			if content, ok := first["content"].(map[string]any); ok {
				if parts, ok := content["parts"].([]any); ok {
					for _, p := range parts {
						if pm, ok := p.(map[string]any); ok {
							if txt, ok := pm["text"].(string); ok && strings.TrimSpace(txt) != "" {
								return txt, nil
							}
						}
					}
				}
			}
		}
	}
	if out, ok := parsed["output"].(string); ok && strings.TrimSpace(out) != "" {
		return out, nil
	}
	if respObj, ok := parsed["response"].(map[string]any); ok {
		if out, ok := respObj["output"].(string); ok && strings.TrimSpace(out) != "" {
			return strings.TrimSpace(out), nil
		}
	}
	return strings.TrimSpace(string(respBytes)), nil
}

func (s *GeminiService) callGenerateContentWithBody(ctx context.Context, model string, body []byte) (string, error) {
	url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent?key=%s", model, s.apiKey)
	log.Printf("[gemini] using model %s", model)
	log.Printf("[gemini] POST %s", url)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("http error: %w", err)
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read error: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("status %d: %s", resp.StatusCode, strings.TrimSpace(string(respBytes)))
	}

	var parsed map[string]any
	if err := json.Unmarshal(respBytes, &parsed); err != nil {
		return strings.TrimSpace(string(respBytes)), nil
	}
	if cands, ok := parsed["candidates"].([]any); ok && len(cands) > 0 {
		if first, ok := cands[0].(map[string]any); ok {
			if content, ok := first["content"].(map[string]any); ok {
				if parts, ok := content["parts"].([]any); ok {
					for _, p := range parts {
						if pm, ok := p.(map[string]any); ok {
							if txt, ok := pm["text"].(string); ok && strings.TrimSpace(txt) != "" {
								return txt, nil
							}
						}
					}
				}
			}
		}
	}
	return strings.TrimSpace(string(respBytes)), nil
}

func (s *GeminiService) callStreamGenerateContent(ctx context.Context, model, prompt string, onDelta func(string)) (string, error) {
	reqBody := map[string]any{
		"contents": []any{
			map[string]any{
				"role":  "user",
				"parts": []any{map[string]any{"text": prompt}},
			},
		},
		"generationConfig": map[string]any{
			"temperature":     0.6,
			"maxOutputTokens": 1024,
			"topK":            40,
			"topP":            0.9,
		},
	}
	bodyBytes, _ := json.Marshal(reqBody)

	url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:streamGenerateContent?key=%s", model, s.apiKey)
	log.Printf("[gemini] streaming model %s", model)
	log.Printf("[gemini] POST %s", url)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(bodyBytes))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("http error: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("status %d: %s", resp.StatusCode, strings.TrimSpace(string(b)))
	}

	full := strings.Builder{}
	scanner := bufio.NewScanner(resp.Body)
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) == "" {
			continue
		}
		if strings.HasPrefix(strings.ToLower(strings.TrimSpace(line)), "data:") {
			line = strings.TrimSpace(line[5:])
		}
		var obj map[string]any
		if err := json.Unmarshal([]byte(line), &obj); err != nil {
			continue
		}
		if cands, ok := obj["candidates"].([]any); ok && len(cands) > 0 {
			if first, ok := cands[0].(map[string]any); ok {
				if content, ok := first["content"].(map[string]any); ok {
					if parts, ok := content["parts"].([]any); ok {
						for _, p := range parts {
							if pm, ok := p.(map[string]any); ok {
								if txt, ok := pm["text"].(string); ok && txt != "" {
									full.WriteString(txt)
									if onDelta != nil {
										onDelta(txt)
									}
								}
							}
						}
					}
				}
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return full.String(), fmt.Errorf("stream read error: %w", err)
	}
	return full.String(), nil
}

func (s *GeminiService) callStreamGenerateContentWithBody(ctx context.Context, model string, body []byte, onDelta func(string)) (string, error) {
	url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:streamGenerateContent?key=%s", model, s.apiKey)
	log.Printf("[gemini] streaming model %s", model)
	log.Printf("[gemini] POST %s", url)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("http error: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("status %d: %s", resp.StatusCode, strings.TrimSpace(string(b)))
	}

	full := strings.Builder{}
	scanner := bufio.NewScanner(resp.Body)
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) == "" {
			continue
		}
		if strings.HasPrefix(strings.ToLower(strings.TrimSpace(line)), "data:") {
			line = strings.TrimSpace(line[5:])
		}
		var obj map[string]any
		if err := json.Unmarshal([]byte(line), &obj); err != nil {
			continue
		}
		if cands, ok := obj["candidates"].([]any); ok && len(cands) > 0 {
			if first, ok := cands[0].(map[string]any); ok {
				if content, ok := first["content"].(map[string]any); ok {
					if parts, ok := content["parts"].([]any); ok {
						for _, p := range parts {
							if pm, ok := p.(map[string]any); ok {
								if txt, ok := pm["text"].(string); ok && txt != "" {
									full.WriteString(txt)
									if onDelta != nil {
										onDelta(txt)
									}
								}
							}
						}
					}
				}
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return full.String(), fmt.Errorf("stream read error: %w", err)
	}
	return full.String(), nil
}

func isRetriable(err error) bool {
	if err == nil {
		return false
	}
	e := strings.ToLower(err.Error())
	if strings.Contains(e, "status 503") || strings.Contains(e, "unavailable") {
		return true
	}
	if strings.Contains(e, "status 429") || strings.Contains(e, "resource_exhausted") || strings.Contains(e, "quota") {
		return true
	}
	return false
}

func sleepWithContext(ctx context.Context, d time.Duration) {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-t.C:
	case <-ctx.Done():
	}
}

func resolveUniversityAlias(key string) string {
	if val, ok := universityAliasMap[key]; ok {
		return val
	}
	return ""
}

func sanitizeUniversityName(raw string) string {
	cleaned := strings.TrimSpace(raw)
	if cleaned == "" {
		return ""
	}
	cleaned = strings.Trim(cleaned, "`\"")
	cleaned = strings.TrimSpace(cleaned)
	cleaned = strings.TrimRight(cleaned, ".,;:!?")
	cleaned = strings.ReplaceAll(cleaned, "\n", " ")
	cleaned = strings.ReplaceAll(cleaned, "\r", " ")
	cleaned = strings.Join(strings.Fields(cleaned), " ")
	upper := strings.ToUpper(cleaned)
	if alias := resolveUniversityAlias(upper); alias != "" {
		return alias
	}
	if upper == "NONE" || upper == "UNKNOWN" || upper == "TIDAK ADA" {
		return ""
	}
	return cleaned
}

func extractUniversityHeuristic(inputs ...string) string {
	for _, text := range inputs {
		t := strings.TrimSpace(text)
		if t == "" {
			continue
		}
		upper := strings.ToUpper(t)
		for aliasKey, aliasVal := range universityAliasMap {
			if strings.Contains(upper, aliasKey) {
				return aliasVal
			}
		}
		if match := universityRegex.FindString(t); match != "" {
			if candidate := sanitizeUniversityName(match); candidate != "" {
				return candidate
			}
		}
	}
	return ""
}

func extractUniversityFromModelResponse(raw string) string {
	if strings.TrimSpace(raw) == "" {
		return ""
	}
	stripped := strings.TrimSpace(raw)
	stripped = strings.Trim(stripped, "`")

	if idx := strings.Index(stripped, "{"); idx >= 0 {
		if end := strings.LastIndex(stripped, "}"); end > idx {
			candidateJSON := stripped[idx : end+1]
			var payload struct {
				University string `json:"university"`
			}
			if err := json.Unmarshal([]byte(candidateJSON), &payload); err == nil {
				if name := sanitizeUniversityName(payload.University); name != "" {
					return name
				}
			}
		}
	}

	if idx := strings.Index(stripped, "\n"); idx > 0 {
		stripped = stripped[:idx]
	}
	return sanitizeUniversityName(stripped)
}

// DetectUniversityName attempts to infer the university name discussed in the chat using Gemini.
// If Gemini is unavailable, it falls back to heuristic detection based on the chat content.
func (s *GeminiService) DetectUniversityName(ctx context.Context, chat []ChatMessage, assistantReply string) (string, error) {
	fallback := extractUniversityHeuristic(assistantReply)
	for i := len(chat) - 1; i >= 0 && fallback == ""; i-- {
		if strings.EqualFold(chat[i].Role, "user") {
			if candidate := extractUniversityHeuristic(chat[i].Text); candidate != "" {
				fallback = candidate
			}
			break
		}
	}

	if config.IsStaging || (config.IsProduction && !config.IsGeminiEnabled) {
		return fallback, nil
	}

	if !s.enabled || strings.TrimSpace(s.apiKey) == "" {
		return fallback, nil
	}

	latestUser := ""
	var convoBuilder strings.Builder
	for _, msg := range chat {
		role := strings.ToLower(strings.TrimSpace(msg.Role))
		if role == "" {
			role = "user"
		}
		if role == "user" {
			latestUser = msg.Text
		}
		convoBuilder.WriteString(strings.ToUpper(role[:1]))
		if len(role) > 1 {
			convoBuilder.WriteString(role[1:])
		}
		convoBuilder.WriteString(": ")
		convoBuilder.WriteString(strings.TrimSpace(msg.Text))
		convoBuilder.WriteString("\n")
	}
	if assistantReply != "" {
		convoBuilder.WriteString("Assistant: ")
		convoBuilder.WriteString(strings.TrimSpace(assistantReply))
		convoBuilder.WriteString("\n")
	}

	if strings.TrimSpace(latestUser) == "" {
		return fallback, nil
	}

	prompt := fmt.Sprintf(`Analisis percakapan berikut dan tentukan apakah ada universitas spesifik yang diminta atau dibahas.

Balas dalam format JSON satu baris seperti berikut:
{"university": "Nama Universitas"}
Jika tidak ada universitas spesifik, gunakan:
{"university": "NONE"}

Percakapan:
%s`, convoBuilder.String())

	models := []string{config.GeminiModel, "gemini-2.0-flash"}
	for _, model := range models {
		if strings.TrimSpace(model) == "" {
			continue
		}
		response, err := s.callGenerateContent(ctx, model, prompt)
		if err != nil {
			if isRetriable(err) {
				sleepWithContext(ctx, 1500*time.Millisecond)
				response, err = s.callGenerateContent(ctx, model, prompt)
			}
		}
		if err != nil {
			continue
		}
		if detected := extractUniversityFromModelResponse(response); detected != "" {
			return detected, nil
		}
	}

	return fallback, nil
}

// AskCampusWithUIBContext asks Gemini with enhanced UIB context for better UIB-related responses
func (s *GeminiService) AskCampusWithUIBContext(ctx context.Context, chat []ChatMessage) (string, error) {
	if !s.enabled {
		log.Printf("[gemini] disabled via config (IsGeminiEnabled=false)")
		return "", ErrGeminiDisabled
	}
	if strings.TrimSpace(s.apiKey) == "" {
		log.Printf("[gemini] GEMINI_API_KEY is not set")
		return "", fmt.Errorf("GEMINI_API_KEY is not set")
	}

	models := []string{config.GeminiModel, "gemini-2.0-flash"}
	tried := make(map[string]error)

	payloadBuilder := func() ([]byte, error) {
		contents := make([]any, 0, len(chat)+2) // +2 for potential UIB context

		// Check if any message in the chat is UIB-related
		var isUIBRelated bool
		var latestUserMessage string

		for _, m := range chat {
			if strings.ToLower(strings.TrimSpace(m.Role)) == "user" {
				latestUserMessage = m.Text
				if s.uibService != nil && s.uibService.AnalyzeQueryForUIB(m.Text) {
					isUIBRelated = true
				}
			}
		}

		// Add UIB context at the beginning if UIB-related
		if isUIBRelated && s.uibService != nil {
			log.Printf("[gemini] ✅ CHAT: UIB context detected! Latest message: %s", latestUserMessage)
			relevantEvents := s.uibService.GetRelevantEventsForQuery(latestUserMessage)
			log.Printf("[gemini] CHAT: Found %d relevant UIB events for context", len(relevantEvents))
			uibContext := s.uibService.FormatEventsForGemini(relevantEvents)

			// Add UIB context as system message
			contents = append(contents, map[string]any{
				"role":  "model",
				"parts": []any{map[string]any{"text": "Saya memiliki akses ke data resmi UIB terbaru untuk tahun 2025. Hari ini tanggal 4 Oktober 2025. Berikut adalah data yang relevan:"}},
			})
			contents = append(contents, map[string]any{
				"role":  "user",
				"parts": []any{map[string]any{"text": uibContext}},
			})
			contents = append(contents, map[string]any{
				"role":  "model",
				"parts": []any{map[string]any{"text": "Data UIB lengkap untuk Oktober-Desember 2025 telah dimuat dengan mark UIB_OFFICIAL. Saya akan langsung memberikan SEMUA data yang tersedia tanpa meminta klarifikasi tambahan. Semua acara bisa didaftarkan sekarang."}},
			})
		}

		// Add all chat messages
		for _, m := range chat {
			role := strings.ToLower(strings.TrimSpace(m.Role))
			if role != "user" && role != "model" {
				role = "user"
			}
			contents = append(contents, map[string]any{
				"role":  role,
				"parts": []any{map[string]any{"text": m.Text}},
			})
		}

		systemInstruction := "Anda adalah asisten kampus yang sangat membantu. Jawab secara rinci, terstruktur (gunakan poin-poin atau langkah), dan jelas dalam Bahasa Indonesia. Jika konteks tidak cukup, minta klarifikasi singkat. Tetap fokus pada topik akademik/kampus."

		if isUIBRelated {
			systemInstruction = `TANGGAL HARI INI: 4 Oktober 2025

Anda adalah asisten resmi Universitas Internasional Batam (UIB). 

INSTRUKSI KHUSUS UIB:
1. PENTING: Hari ini adalah 4 Oktober 2025, jadi semua acara Oktober-Desember 2025 adalah SAAT INI atau AKAN DATANG
2. LANGSUNG berikan SEMUA data yang tersedia - JANGAN tanya balik atau minta klarifikasi
3. Jika ditanya tentang sertifikasi/webinar per bulan, tampilkan SEMUA yang ada di bulan tersebut
4. WAJIB gunakan data UIB yang telah disediakan sebagai sumber utama
5. Format: "Berikut sertifikasi UIB untuk [bulan]:" lalu list semua dengan detail lengkap
6. SELALU sebutkan bahwa informasi berasal dari data resmi UIB (UIB_OFFICIAL)
7. Sertakan detail lengkap: tanggal, waktu, lokasi, biaya, kontak jika tersedia
8. JANGAN katakan "memerlukan informasi lebih lanjut" - langsung berikan semua yang ada
9. Format jawaban dengan emoji dan struktur yang menarik
10. Untuk pendaftaran, selalu sertakan informasi kontak dan deadline jika ada

Prioritas jawaban: Data UIB lengkap → Informasi umum kampus → Saran kontak UIB`
		}

		reqBody := map[string]any{
			"systemInstruction": map[string]any{
				"parts": []any{map[string]any{"text": systemInstruction}},
			},
			"contents": contents,
			"generationConfig": map[string]any{
				"temperature":     0.6,
				"maxOutputTokens": 2048,
				"topK":            40,
				"topP":            0.9,
			},
		}
		return json.Marshal(reqBody)
	}

	for _, m := range models {
		if strings.TrimSpace(m) == "" {
			continue
		}
		payload, err := payloadBuilder()
		if err != nil {
			return "", fmt.Errorf("failed to build payload: %w", err)
		}
		text, err := s.callGenerateContentWithBody(ctx, m, payload)
		if err != nil && isRetriable(err) {
			sleepWithContext(ctx, 2*time.Second)
			text, err = s.callGenerateContentWithBody(ctx, m, payload)
		}
		if err == nil && strings.TrimSpace(text) != "" {
			return strings.TrimSpace(text), nil
		}
		if err != nil {
			tried[m] = err
			log.Printf("[gemini] model %s failed: %v", m, err)
		}
	}

	var b strings.Builder
	b.WriteString("all gemini models failed: ")
	first := true
	for m, e := range tried {
		if !first {
			b.WriteString("; ")
		}
		first = false
		b.WriteString(fmt.Sprintf("%s -> %v", m, e))
	}
	return "", errors.New(b.String())
}
