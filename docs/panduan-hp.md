# Panduan PixelHop dari HP: lampiran keluhan jadi link

Untuk CS yang menerima bukti dari pelanggan (screenshot, rekaman layar, video ATM, PDF mutasi) dan perlu meneruskannya ke admin lewat WhatsApp atau LINE sebagai **link**, bukan file.

## Sekali saja: persiapan

1. Buka alamat PixelHop kamu di HP (misalnya **https://p.hel.ink**) dan masuk ke akunmu. Di p.hel.ink, login memakai akun HEL.ink.
2. **Android (Chrome):** menu ⋮ → **Tambahkan ke layar utama / Install aplikasi**. Ikon PixelHop muncul di layar HP.
3. **iPhone:** ikuti bagian [iPhone](#iphone) di bawah.
4. Minta admin menaikkan kuota penyimpanan akun CS (bawaan 1 GB) kalau sering menerima video.

## Cara 1: langsung dari WhatsApp / LINE (paling cepat)

1. Di chat pelanggan, **tekan lama** gambar, video atau PDF. Bisa pilih beberapa sekaligus.
2. Tekan **Bagikan / Share**, lalu pilih **PixelHop**.
3. Tunggu sampai halaman "Uploaded" muncul.
4. Tekan **Copy for chat**, lalu tempel di chat admin.

## Cara 1b: dari aplikasi yang tidak punya tombol Share (copy–paste)

1. Di aplikasi chat kerja, tekan lama gambar dan pilih **Salin / Copy**.
2. Buka PixelHop, lalu tekan tombol **Paste** di sebelah *Choose files*.
3. Pertama kali, Chrome menanyakan izin membaca clipboard: pilih **Izinkan**. Gambar langsung terupload.
4. Kalau yang muncul justru kotak "Press and hold here, then tap Paste", tekan lama kotak itu lalu pilih **Tempel / Paste**.

## Cara 2: dengan nomor tiket dan catatan (paling rapi)

1. Buka aplikasi PixelHop (harus sudah login). Di bawah tombol upload ada kotak **Ticket / case**.
2. Isi **nomor tiket atau transaksi**, misalnya `TRX-20261016-0042`, dan **catatan singkat**, misalnya "TF berhasil, status di aplikasi gagal".
3. Di **Delete after**, pilih masa simpan. Saran: **90 days**.
4. Tekan **Choose files** dan pilih semua bukti. Video besar diupload sedikit demi sedikit; kalau sinyal putus, upload lanjut sendiri.
5. Tekan **Copy for chat**. Hasil yang ditempel ke admin:

```
TRX-20261016-0042
TF berhasil, status di aplikasi gagal
All files: https://p.hel.ink/a/xxxxxx

1. rekaman-layar: https://p.hel.ink/rekaman-layar-AbC123
2. mutasi: https://p.hel.ink/mutasi-XyZ789
```

Admin cukup buka satu link album untuk melihat semuanya: video bisa diputar dan PDF bisa dibuka langsung di HP.

## Yang terjadi otomatis

- **Video** langsung bisa ditonton lewat link. Di belakang layar, video diperkecil, misalnya rekaman 50 MB menjadi sekitar 2 MB, supaya cepat dibuka di HP admin.
- **PDF** dikompres. Halaman pertamanya tampil sebagai pratinjau.
- **Lokasi GPS** dan data tersembunyi lain di foto dan video dihapus.
- Batas ukuran (sudah login): gambar 20 MB, video dan PDF 100 MB per file. Tanpa login: semua file maksimal 20 MB.

## Privasi

Bukti transaksi berisi data pribadi. Biasakan hal berikut:
- Jangan centang **Show on my profile**.
- Kalau perlu, buka halaman file, tekan **Edit details**, lalu pilih **password** atau **Only me**.
- Pakai masa simpan (**Delete after**) supaya bukti terhapus sendiri setelah kasus selesai.

## iPhone

Safari di iPhone tidak bisa menambahkan website ke menu Share. Karena itu, pakai **Shortcut**; cukup dipasang sekali.

1. Di p.hel.ink buka **Settings → API keys**, buat key, lalu salin (diawali `ph_`).
2. Buka aplikasi **Shortcuts**, tekan **+**, dan beri nama **Upload ke PixelHop**.
3. Tekan ikon ⓘ dan nyalakan **Show in Share Sheet**. Terima: Images, Media, PDFs, Files.
4. Tambahkan action **Repeat with Each** untuk *Shortcut Input*. Di dalamnya:
   - **Get Contents of URL**:
     - URL `https://p.hel.ink/api/v1/upload`
     - Method **POST**
     - Header `Authorization` = `Bearer ph_…` (key tadi)
     - Request Body **Form**, field **File** bernama `file` = *Repeat Item*
   - **Get Dictionary Value** `data`
   - **Get Dictionary Value** `view_url`
5. Setelah **End Repeat**, tambahkan:
   - **Combine Text** (*Repeat Results*, pemisah *New Lines*);
   - **Copy to Clipboard**;
   - **Show Notification** "Link disalin".

Setelah itu, di WhatsApp/LINE tekan file, lalu **Share → Upload ke PixelHop**, dan link-nya langsung tersalin. Lewat Shortcut, maksimal 100 MB per file. Untuk video yang lebih besar, pakai **Cara 2** di website.
