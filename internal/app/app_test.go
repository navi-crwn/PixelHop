package app

import "testing"

func TestPasswordsFromPHP(t *testing.T) {
	// password_hash('secret123', PASSWORD_ARGON2ID) from PHP 8.
	php := "$argon2id$v=19$m=65536,t=4,p=1$WGFMcDlkc3J1WkE5UnBmSA$Rs655OsQWxGJV0kpBKlAXds9shcFBQNqODTLZdA+M2g"
	if !CheckPassword(php, "secret123") {
		t.Fatal("PHP argon2id hash should verify")
	}
	if CheckPassword(php, "wrong") {
		t.Fatal("wrong password accepted")
	}
	h := HashPassword("hello world")
	if !CheckPassword(h, "hello world") || CheckPassword(h, "hello") {
		t.Fatal("own hashes broken")
	}
	if CheckPassword("", "") || CheckPassword("$argon2id$garbage", "x") {
		t.Fatal("garbage hash accepted")
	}
}

func TestSlugAndIDs(t *testing.T) {
	cases := map[string]string{
		"Rumah Baru":                "rumah-baru",
		"IMG_20240101_123456":       "",
		"WhatsApp Image 2024-01-01": "",
		"   ---  ":                  "",
		"a very long holiday photo name that keeps going": "a-very-long-holiday",
		"café ☕ menu": "caf-menu",
	}
	for in, want := range cases {
		if got := slugify(in); got != want {
			t.Errorf("slugify(%q) = %q, want %q", in, got, want)
		}
	}
	id := newID("beach")
	if len(id) != len("beach-")+6 || !validID(id) {
		t.Fatalf("bad id %q", id)
	}
	for _, bad := range []string{"", "../etc", "a/b", "x.y", "<script>"} {
		if validID(bad) {
			t.Errorf("validID(%q) should be false", bad)
		}
	}
}

func TestSafeNext(t *testing.T) {
	for in, want := range map[string]string{"/admin": "/admin", "//evil.com": "/dashboard", "https://evil.com": "/dashboard", "/\\evil": "/dashboard", "": "/dashboard"} {
		if got := safeNext(in); got != want {
			t.Errorf("safeNext(%q) = %q", in, got)
		}
	}
}
