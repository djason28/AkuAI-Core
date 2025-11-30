# Rencana Sprint AkuAI-Core (6 Sprint)

Dokumen ini merinci enam sprint yang selaras dengan kondisi aktual repositori AkuAI-Core. Fokusnya menjaga kesesuaian (no over-claim), menekankan inkremental delivery, dan menyertakan acceptance criteria yang dapat diverifikasi pada repo saat ini.

## Ringkasan Sprint (2 minggu per sprint)

| Sprint | Durasi | Fokus / Tujuan | Deliverable Inti |
|-------:|:------:|----------------|------------------|
| 1 | 2 minggu | Inisiasi proyek & setup environment | Struktur repo backend/frontend, .env.example, konfigurasi runtime, sprint plan (dokumen ini) |
| 2 | 2 minggu | Core backend: DB, model, auth, UIB events, cache dasar, unit tests | Endpoint autentikasi, percakapan dasar, UIB events API, cache chat dasar, unit test rate limit & cache |
| 3 | 2 minggu | Frontend minimal & integrasi E2E | UI login/register, UI chat dasar (WS/REST), profil dasar, integrasi ke backend |
| 4 | 2 minggu | Integrasi AI (Gemini), prompt templating, streaming & logging terkendali | Layanan Gemini + UIB context injection, mode mock/fallback, streaming response, prompt logging (AB test context) |
| 5 | 2 minggu | Eksperimen prompt & evaluasi batch | CLI A/B test, hasil CSV/JSON, skor sederhana, catatan evaluasi |
| 6 | 2 minggu | Stabilisasi, security minimal, dokumentasi final | JWT + rate limit aktif, pengendalian duplikasi, hardening ringan, dokumentasi akhir; (opsional) starter CI & Docker sebagai backlog lanjutan |

> Catatan penting:
> - Tidak mengklaim Docker environment atau CI sudah ada. Keduanya disiapkan sebagai backlog opsional pada Sprint 6/lanjutan.
> - Prompt-logs tersedia dalam konteks AB test dan mencatat hash prompt/context; penyimpanan pertanyaan mentah digunakan untuk analisis offline. Redaksi PII untuk production logging diletakkan sebagai backlog Sprint 6.

---

## Sprint 1 — Inisiasi proyek & setup environment (2 minggu)

- Fokus
  - Menetapkan struktur proyek backend (Go + Gin) dan frontend (SvelteKit)
  - Menyediakan konfigurasi environment yang jelas, aman, dan mudah dijalankan lokal
  - Menetapkan artefak Scrum: sprint plan (dokumen ini)

- Deliverables
  - Struktur repo: `core/` (backend), `views/` (frontend)
  - `.env.example` dan loader konfigurasi: `core/pkg/config/config.go`
  - Sprint plan: `core/docs/sprint-plan.md` (dokumen ini)
  - README run steps (backend): `core/README.md`

- Acceptance criteria
  - Backend dapat dibangun: `go build` sukses pada `core/`
  - Environment terbaca dari `.env` saat `APP_ENV=staging` (local), dan wajib variabel kritikal saat `production`
  - Dokumen sprint plan tersedia di repo

---

## Sprint 2 — Core backend (DB, model, auth, UIB events, cache, tests) (2 minggu)

- Fokus
  - Koneksi MySQL + GORM, model utama (users, conversations, messages)
  - Autentikasi JWT, routing modular, rate limiting dasar
  - API UIB events dengan data resmi (file JSON) + layanan pencarian/formatting
  - Cache chat sederhana dengan kebijakan status-aware

- Deliverables (contoh path)
  - Model & DB: `core/models/*.go`, koneksi di `core/main.go`
  - Auth & token: `core/controllers/auth.go`, `core/pkg/token/tokenstore.go`, rute di `core/routes/auth/*`
  - UIB events API: `core/pkg/services/uib_service.go`, rute di `core/routes/uib/uib.go`, data `core/data/uib_events.json`
  - Rate limiting: `core/middleware/ratelimit.go` (+ test)
  - Cache chat: `core/pkg/cache/*` (+ test)

- Acceptance criteria
  - Endpoint auth (register/login) berfungsi dan mengembalikan JWT
  - UIB events endpoint mengembalikan data sesuai filter (bulan/tipe/dll.)
  - Unit test lulus untuk rate limit dan cache

---

## Sprint 3 — Frontend minimal & integrasi end-to-end (2 minggu)

- Fokus
  - UI login/register, chat dasar (WebSocket/REST), profil sederhana
  - Integrasi ke backend (CORS, token storage, error state)

- Deliverables (contoh path)
  - SvelteKit routes: `views/src/routes/*` (login, register, chat, layout)
  - Store auth & util: `views/src/lib/stores/auth.js`, `views/src/utils/*`
  - Integrasi WS: `core/controllers/ws.go`, `core/routes/websocket/*`

- Acceptance criteria
  - Pengguna dapat login dari UI dan melakukan chat (kirim/terima) end-to-end
  - Error/fallback state tampil di UI saat layanan eksternal dimock/timeout

---

## Sprint 4 — Integrasi AI (Gemini), prompt templating, streaming & logging (2 minggu)

- Fokus
  - Layanan Gemini dengan fallback/mock untuk ketersediaan
  - Prompt engineering dinamis: injeksi konteks UIB events ke prompt
  - Respons streaming dan logging terkontrol untuk replikasi eksperimen

- Deliverables (contoh path)
  - Gemini service: `core/pkg/services/gemini.go` (Ask/Stream/WithChat, retry, multi-model)
  - Prompt/context builder: `core/pkg/services/uib_service.go` (FormatEventsForGemini, deteksi query UIB)
  - Prompt logs (AB test context): JSONL via `appendPromptLog` ketika `log_file` diset di context

- Acceptance criteria
  - Pertanyaan UIB menyuntikkan konteks acara ke prompt (terlihat di log debug)
  - Saat staging atau layanan dimatikan, sistem mengembalikan mock response
  - Streaming teks berfungsi dan fallback ke non-stream jika gagal

> Catatan PII: logging penuh prompt/context hanya aktif ketika diizinkan variabel lingkungan AB test; hash digunakan untuk identifikasi ringkas. Redaksi PII untuk production logging direncanakan pada Sprint 6.

---

## Sprint 5 — Eksperimen prompt & evaluasi batch (2 minggu)

- Fokus
  - Menjalankan A/B test batch, menyimpan hasil, dan mengevaluasi kualitas respons

- Deliverables (contoh path)
  - Tool AB test: `core/cmd/abtest/*`, skor: `core/cmd/abscore/*`
  - Hasil terarsip: `core/cmd/abtest/results/*` (CSV/JSON, prompt_logs opsional)
  - Dokumentasi evaluasi: `core/docs/bab-4.6-evaluasi-abtest.md`

- Acceptance criteria
  - Dapat menjalankan batch query dan menyimpan hasil (JSON/CSV)
  - Skor/analisis sederhana dapat dihasilkan dan didokumentasikan

---

## Sprint 6 — Stabilisasi, security minimal, dokumentasi akhir (2 minggu)

- Fokus
  - Hardening ringan: autentikasi, rate limiting, proteksi duplikasi, konfigurasi produksi
  - Dokumentasi akhir untuk deployment dan eksperimen
  - Backlog opsional: starter CI build/test dan Docker runtime

- Deliverables (contoh path)
  - Security minimal aktif: JWT, rate limit, duplicate request guard (konfigurasi di `core/pkg/config/config.go`)
  - Dokumentasi final/operasional: `core/README.md`, `core/docs/*`
  - (Opsional backlog) CI: workflow build/test; Docker: `Dockerfile`/`compose.yml` — tidak diklaim selesai pada kondisi repo saat ini
  - (Opsional backlog) Redaksi PII untuk production prompt-logs (regex email/no telp/NIM) sebelum persist

- Acceptance criteria
  - Build backend sukses di mode produksi, variabel kritikal wajib di-set
  - Rate limiting dan autentikasi aktif pada endpoint yang sesuai
  - Dokumentasi “cara jalan” dan “eksperimen” up to date
  - (Jika diambil) CI mengeksekusi `go build` + `go test` paket kritikal; Docker image backend dapat dibangun dan dijalankan

---

## Backlog Lanjutan (di luar 6 sprint atau opsional Sprint 6)
- CI minimal (GitHub Actions) untuk build/test Go dan lint/build SvelteKit
- Dockerfile + compose untuk backend + MySQL + SvelteKit
- Redaksi PII otomatis untuk production prompt-logs
- Monitoring ringan (health check endpoint + logging terstruktur ke target eksternal)

## Pemetaan Direktori Singkat
- Backend: `core/controllers/*`, `core/routes/*`, `core/middleware/*`, `core/pkg/services/*`, `core/pkg/cache/*`, `core/models/*`
- Frontend: `views/src/routes/*`, `views/src/lib/*`, `views/src/utils/*`
- Data & Docs: `core/data/uib_events.json`, `core/docs/*`
