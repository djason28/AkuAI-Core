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
	"strings"
	"time"

	"AkuAI/pkg/config"
)

type GeminiService struct {
	apiKey  string
	enabled bool
}

var (
	ErrGeminiDisabled = errors.New("gemini is disabled via config")
)

func NewGeminiService() *GeminiService {
	return &GeminiService{
		apiKey:  config.GeminiAPIKey,
		enabled: config.IsGeminiEnabled,
	}
}

type ChatMessage struct {
	Role string
	Text string
}

func (s *GeminiService) AskCampus(ctx context.Context, question string) (string, error) {
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
				"parts": []any{map[string]any{"text": "Anda adalah asisten kampus yang sangat membantu. Jawab secara rinci, terstruktur (gunakan poin-poin atau langkah), dan jelas dalam Bahasa Indonesia. Jika konteks tidak cukup, minta klarifikasi singkat. Tetap fokus pada topik akademik/kampus."}},
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
				"parts": []any{map[string]any{"text": "Anda adalah asisten kampus yang sangat membantu. Jawab secara rinci, terstruktur (gunakan poin-poin atau langkah), dan jelas dalam Bahasa Indonesia. Jika konteks tidak cukup, minta klarifikasi singkat. Tetap fokus pada topik akademik/kampus."}},
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
