# 📧 Cloudflare Email Routing Setup untuk PixelHop

## Email Addresses yang Direkomendasikan

| Email | Forward ke | Kegunaan |
|-------|-----------|----------|
| `no-reply@p.hel.ink` | no-reply@hel.ink | Verification emails, password reset |
| `contact@p.hel.ink` | email pribadi | General inquiries |
| `abuse@p.hel.ink` | email pribadi | DMCA, abuse reports |
| `support@p.hel.ink` | email pribadi | User support |

## Setup di Cloudflare

### 1. Buka Email Routing
1. Login ke [Cloudflare Dashboard](https://dash.cloudflare.com)
2. Pilih domain `p.hel.ink`
3. Klik **Email** → **Email Routing**

### 2. Enable Email Routing
- Klik **Enable Email Routing**
- Cloudflare akan otomatis add MX records

### 3. Add Destination Email
- Klik **Destination addresses**
- Add email pribadi Anda sebagai destination
- Verify email tersebut (cek inbox untuk verification link)

### 4. Create Routing Rules
Buat rules untuk forward email:

**Rule 1: Contact**
- Custom address: `contact`
- Action: Forward to → email pribadi

**Rule 2: Abuse**
- Custom address: `abuse`
- Action: Forward to → email pribadi

**Rule 3: Support**
- Custom address: `support`
- Action: Forward to → email pribadi

**Rule 4: Catch-all (optional)**
- Catch-all address
- Action: Forward to → email pribadi
- Ini akan catch semua email yang tidak match rules lain

### 5. Verify MX Records
Pastikan MX records sudah benar:
```
MX  p.hel.ink  route1.mx.cloudflare.net  Priority: 69
MX  p.hel.ink  route2.mx.cloudflare.net  Priority: 69
MX  p.hel.ink  route3.mx.cloudflare.net  Priority: 69
```

## ⚠️ Catatan Penting

### Sending Emails (SMTP)
Cloudflare Email Routing **HANYA untuk menerima email**, bukan mengirim.

Untuk **mengirim email** (verification, password reset), Anda tetap perlu:
- SMTP server (seperti yang sudah ada: `mail.spacemail.com`)
- Atau gunakan layanan transactional email seperti:
  - Mailgun
  - SendGrid
  - Amazon SES
  - Resend.com (recommended, gratis 3000 email/bulan)

### Current SMTP Config
Dari `/config/oauth.php`:
```php
'smtp' => [
    'host' => 'mail.spacemail.com',
    'port' => 465,
    'username' => 'no-reply@hel.ink',
    'from_email' => 'no-reply@hel.ink',
    'encryption' => 'ssl',
],
```

Ini sudah benar untuk sending emails.

## SPF, DKIM, DMARC Records

Untuk deliverability yang baik, pastikan DNS records ini ada:

### SPF Record
```
TXT  p.hel.ink  v=spf1 include:_spf.mx.cloudflare.net include:spacemail.com ~all
```

### DMARC Record
```
TXT  _dmarc.p.hel.ink  v=DMARC1; p=quarantine; rua=mailto:abuse@p.hel.ink
```

### DKIM
Biasanya di-setup oleh email provider (spacemail.com).

---

## Free Tier Limits

| Feature | Limit |
|---------|-------|
| Custom addresses | 25 |
| Destination addresses | 200 |
| Email size | 25 MB |
| Daily emails | Unlimited (for routing) |
