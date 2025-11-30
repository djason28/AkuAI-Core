# Bab 4.6 — Evaluasi A/B Prompting dan Reliabilitas Output

Dokumen ini meringkas metode evaluasi, metrik, signifikansi statistik, dan praktik reproduksibilitas untuk A/B test antara dua strategi prompt:
- baseline: AskCampus (tanpa injeksi konteks UIB eksplisit)
- engineered: AskCampusWithChat (deteksi intent UIB + injeksi konteks event UIB)

Ringkasan ini adalah versi terbaru (per 26 Okt 2025) dan mencakup pembaruan: recall berbasis ID, kuantifikasi hallucination, pemeriksaan format, uji berpasangan, serta metadata reproduksibilitas.

## 1) Dataset & Prosedur
- Query set: 20 pertanyaan tematik (event, per-bulan, minggu depan, 3 event, gambar, kontak/tautan).
- Dua mode per query: baseline vs engineered.
- Service: Gemini, temperature tetap (deterministik jika 0), throttling ringan, retry jika 429 (hormat "Please retry in Xs").
- Output: JSON dan CSV berstempel waktu di `core/cmd/abtest/results/`.

Menjalankan runner (Windows PowerShell):

```powershell
# Dari folder repo root
cd .\core
$env:APP_ENV="staging"; $env:ABTEST_TIMEOUT_SEC="40"; go run ./cmd/abtest
```

## 2) Reproducibility
Untuk setiap run kami menyimpan:
- run_id, random_seed, model, temperature
- prompt_template_id, prompt_template_version
- context_hash dan opsional context_snapshot
- prompt logs (JSONL) dengan prompt_id, sistem prompt, dan snapshot jika diaktifkan

Hasil A/B terakhir dapat di-skor ulang dengan:

```powershell
cd .\core
$env:ABTEST_RESULTS="cmd/abtest/results/abtest-YYYYmmdd-HHMMSS.json"; go run ./cmd/abscore
```

## 3) Metrik
- Coverage/Recall (by ID):
  - Jika `relevant_event_ids` tersedia di hasil, recall dihitung secara ketat berdasarkan ID (bukan substring judul).
- Precision & F1:
  - Precision dihitung dari TP/(TP+FP) berdasarkan judul yang terdeteksi; saat model mengeluarkan ID terstruktur, precision dihitung dari ID.
- Format compliance (rule-based):
  - Pemeriksaan “Tanggal/Lokasi”, heading bulan, jumlah item (mis. 3 event), aturan gambar (3 URL + “sumber”), serta placeholder saat data tidak tersedia.
- Hallucination:
  - FP events (judul/ID yang tak relevan);
  - Fabricated contact/link: alamat email @uib.ac.id dan tautan yang tidak ada dalam ground truth (harus memakai placeholder jika tidak tersedia).
- Per-item TP/FP/FN labeling:
  - Disimpan untuk analisis granular dan fabricated_event_rate per mode.

Artefak keluaran scorer:
- `score-*.csv`: ringkasan per baris (query, mode, coverage, precision, f1, format_ok, flags)
- `score-items-*.csv`: label TP/FP/FN per event

## 4) Signifikansi Statistik
- Wilcoxon signed-rank pada F1 (paired per-query)
- McNemar (exact/binomial) pada biner fabricated_any dan format_pass
- Catatan: Sampel 20 biasanya underpowered untuk mendeteksi efek kecil; gunakan ≥50–100 query untuk bukti lebih kuat.

Opsional (lanjutan, direkomendasikan):
- Bootstrap CI 95% untuk precision/recall/F1
- Paired permutation test untuk delta F1
- Holm–Bonferroni untuk koreksi multi-metrik

## 5) Hasil Terbaru (Singkat)
- Skor berjalan otomatis dari file hasil terpilih (lihat log terminal saat menjalankan `abscore`).
- Ringkasan rata-rata precision/coverage/F1, format_pass, serta fabricated_contact/link ditampilkan; CSV hasil disimpan di `core/cmd/abtest/results/` dengan prefiks `score-`.
- Dengan recall-ID yang ketat, interpretasi menjadi lebih andal dibanding matching judul.

Catatan: Jika Anda telah melakukan run dengan file terbaru, jalankan kembali `abscore` untuk melihat agregat dan file CSV teranyar.

## 6) Praktik Terbaik untuk Mengurangi Hallucination
- Wajibkan output JSON terstruktur (schema):
  - events: [{id, title, date, location, contact, link, placeholder}]
  - link dan contact hanya diisi jika tersedia di data; jika tidak, `placeholder=true` dan nilai `null`.
- Gunakan ID event di output (bukan hanya judul) agar scoring dan validasi faktual presisi.
- Validasi ketat:
  - JSON schema validation (fail closed jika invalid)
  - Field-level accuracy: tanggal/lokasi/kontak/tautan dicocokkan terhadap ground truth untuk ID yang sama
- Aturan placeholder dan allowlist:
  - Hanya alamat email @uib.ac.id dari dataset yang boleh muncul; selain itu wajib placeholder
  - Tautan opsional; jika tidak ada, jangan mengarang, gunakan placeholder
- Evidence/provenance:
  - Minta `evidence_ids` (ID yang digunakan) untuk setiap event yang dioutput
- Prompt guardrails:
  - “Jangan mengarang. Jika field tidak tersedia, set `placeholder=true` dan nilai `null`.”
  - “Hanya keluarkan ID yang terdapat dalam konteks.”

## 7) Batasan & Rencana Lanjutan
- 20 query masih terbatas untuk inferensi statistik; tingkatkan ukuran sampel dan stratifikasi per jenis query
- Tambahkan ingestion skor manusia (relevansi, faithfulness, kegunaan) untuk validasi eksternal; jalankan Wilcoxon/McNemar di skor manusia
- Implementasikan bootstrap CI, permutation test, dan koreksi Holm–Bonferroni di CLI scorer
- Pindahkan pipeline sepenuhnya ke ID-based precision/recall serta field-level accuracy dari output JSON model

## 8) Lokasi File Terkait
- Runner: `core/cmd/abtest`
- Scorer: `core/cmd/abscore`
- Hasil: `core/cmd/abtest/results/abtest-*.json` dan `core/cmd/abtest/results/score-*.csv`
- Prompt logs (opsional): `core/cmd/abtest/results/prompt-logs-*.jsonl`

---
Reproducibility: semua run mencantumkan `run_id`, `random_seed`, `model`, `temperature`, `prompt_template_id/version`, dan `context_hash`. Anda dapat menyalin kembali hasil JSON ke mesin lain dan menjalankan `abscore` untuk memperoleh metrik & CSV yang sama.
