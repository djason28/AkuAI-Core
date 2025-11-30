# Prompt Engineering untuk AkuAI Chatbot

## 1. Tujuan Prompt Engineering
Prompt engineering digunakan untuk memastikan model Gemini memberikan respons yang relevan, akurat, dan sesuai konteks kampus UIB. Prompt disusun dengan menyuntikkan data kampus (event, jurusan) ke dalam instruksi yang dikirim ke model.

## 2. Alur Penyusunan Prompt
1. **Ekstraksi Konteks**: Backend mengambil data event UIB (Oktober–Desember 2025) dan, jika ada, daftar jurusan.
2. **Format Data**: Data diformat menjadi string atau JSON ringkas.
3. **Konstruksi Prompt**: Prompt utama berisi instruksi, diikuti data kampus, lalu pertanyaan user.

### Contoh Template Prompt
```
Anda adalah asisten AI untuk Universitas Internasional Batam (UIB). Jawablah pertanyaan berikut berdasarkan data resmi kampus.

Data Event Kampus (Oktober–Desember 2025):
{event_json}

Daftar Jurusan:
{departments_json}

Pertanyaan pengguna:
"{user_question}"

Jawaban:
```

## 3. Contoh Prompt Aktual
```
Anda adalah asisten AI untuk Universitas Internasional Batam (UIB). Jawablah pertanyaan berikut berdasarkan data resmi kampus.

Data Event Kampus (Oktober–Desember 2025):
[
  {"title": "Sertifikasi Digital Marketing for Business", "date": "2025-10-05", ...},
  {"title": "Future of Artificial Intelligence in Education", "date": "2025-10-12", ...}
]

Daftar Jurusan:
[
  {"name": "Teknik Informatika", "desc": "Belajar tentang software dan teknologi informasi."},
  {"name": "Manajemen", "desc": "Fokus pada bisnis dan organisasi."}
]

Pertanyaan pengguna:
"Apa saja webinar yang ada di bulan Oktober?"

Jawaban:
```

## 4. Catatan Eksperimen Prompt
- Prompt dengan instruksi eksplisit dan data terstruktur menghasilkan jawaban lebih relevan.
- Penambahan kata kunci “berdasarkan data resmi kampus” mengurangi hallucination.
- Prompt terlalu panjang dapat dipotong, jadi data diformat ringkas.

## 5. Referensi Kode
- Penyusunan context: `pkg/services/uib_service.go`, `FormatEventsForGemini`
- Integrasi prompt: `pkg/services/gemini.go`, `AskCampusWithUIBContext`
