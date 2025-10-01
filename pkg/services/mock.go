package services

import (
	"context"
	"fmt"
	"math/rand"
	"strings"
	"time"
)

func AskCampusWithChatLocal(ctx context.Context, chat []ChatMessage) string {
	var last string
	if len(chat) > 0 {
		last = strings.TrimSpace(chat[len(chat)-1].Text)
	}
	if last == "" {
		last = "pertanyaan Anda"
	}
	b := &strings.Builder{}
	fmt.Fprintf(b, "Ringkasan jawaban untuk: %s\n\n", last)
	fmt.Fprintln(b, "Rangkuman:")
	fmt.Fprintf(b, "- Topik utama: %s\n", truncate(last, 60))
	fmt.Fprintln(b, "- Konteks: Informasi kampus/akademik")
	fmt.Fprintln(b, "\nDetail langkah:")
	fmt.Fprintln(b, "1) Penjelasan singkat topik terkait.")
	fmt.Fprintln(b, "2) Langkah yang dapat Anda lakukan (contoh praktis).")
	fmt.Fprintln(b, "3) Sumber informasi kampus yang relevan (website resmi, admin prodi, dsb).")
	fmt.Fprintln(b, "\nCatatan:")
	fmt.Fprintln(b, "- Jika butuh data spesifik (tanggal, biaya, syarat), cek laman resmi atau hubungi admin.")
	fmt.Fprintln(b, "- Beri detail tambahan agar jawaban bisa lebih tepat.")
	fmt.Fprintln(b, "\nPertanyaan lanjutan yang disarankan:")
	fmt.Fprintln(b, "- Apakah ada program/jurusan tertentu yang Anda maksud?")
	fmt.Fprintln(b, "- Kapan batas waktu atau periode yang diinginkan?")
	return b.String()
}

func StreamCampusWithChatLocal(ctx context.Context, chat []ChatMessage, onDelta func(string)) string {
	full := AskCampusWithChatLocal(ctx, chat)
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	i := 0
	for i < len(full) {
		if ctx.Err() != nil {
			break
		}
		step := 16 + r.Intn(32)
		if i+step > len(full) {
			step = len(full) - i
		}
		part := full[i : i+step]
		if onDelta != nil {
			onDelta(part)
		}
		i += step
		sleepWithContext(ctx, 40*time.Millisecond)
	}
	return full
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	if n <= 3 {
		return s[:n]
	}
	return s[:n-3] + "..."
}
