package config

import "testing"

const (
	storeOwn  = "store-secret-for-filehop-only-0123456789"
	otherSecr = "some-other-secret-that-is-long-enough-01234"
)

// A store client's secret must be its own: one that equals the SSO, picker or media-client secret
// (or another store client's) leaves that client off, instead of letting a holder of the other secret sign as it.
func TestStoreSecretMustDifferFromOtherSecrets(t *testing.T) {
	cases := map[string]map[string]string{
		"the SSO secret":            {"HELINK_URL": "https://hel.example", "HELINK_SSO_SECRET": storeOwn},
		"the previous SSO secret":   {"HELINK_URL": "https://hel.example", "HELINK_SSO_SECRET": otherSecr, "HELINK_SSO_SECRET_PREV": storeOwn},
		"the shared HOP SSO secret": {"HOP_SSO_SECRET": storeOwn},
		"the app's own SSO secret":  {"HOP_SSO_SECRET_PIXELHOP": storeOwn},
		"the picker secret":         {"HOP_PICKER_SECRET": storeOwn},
		"a media client's secret":   {"PIXELHOP_MEDIA_CLIENTS": "biohop", "HOP_SSO_SECRET_BIOHOP": storeOwn},
		"another store client":      {"PIXELHOP_STORE_CLIENTS": "filehop,payhop", "HOP_STORE_SECRET_PAYHOP": storeOwn},
	}
	for name, env := range cases {
		t.Run(name, func(t *testing.T) {
			t.Setenv("PIXELHOP_STORE_CLIENTS", "filehop")
			t.Setenv("HOP_STORE_SECRET_FILEHOP", storeOwn)
			for k, v := range env {
				t.Setenv(k, v)
			}
			c, err := Load()
			if err != nil {
				t.Fatal(err)
			}
			// For the last case the first listed client (filehop) keeps the secret and the later one is refused.
			if name == "another store client" {
				if !c.StoreClient("filehop") || c.StoreClient("payhop") {
					t.Fatalf("filehop %v payhop %v", c.StoreClient("filehop"), c.StoreClient("payhop"))
				}
				return
			}
			if c.StoreClient("filehop") || c.StoreClientsConfigured() || len(c.StoreClientSecrets("filehop")) != 0 {
				t.Fatal("a store secret equal to " + name + " was accepted")
			}
		})
	}
}

func TestStoreSecretThatIsItsOwnIsAccepted(t *testing.T) {
	t.Setenv("PIXELHOP_STORE_CLIENTS", "filehop")
	t.Setenv("HOP_STORE_SECRET_FILEHOP", storeOwn)
	t.Setenv("HELINK_URL", "https://hel.example")
	t.Setenv("HELINK_SSO_SECRET", otherSecr)
	t.Setenv("HOP_PICKER_SECRET", "a-picker-secret-that-is-long-enough-0123456")
	t.Setenv("HOP_SSO_SECRET_BIOHOP", "a-biohop-secret-that-is-long-enough-0123456")
	c, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if !c.StoreClient("filehop") || len(c.StoreClientSecrets("filehop")) != 1 || c.StoreClientSecrets("filehop")[0] != storeOwn {
		t.Fatal("a distinct store secret was not accepted")
	}
	if c.StoreClient("biohop") || len(c.StoreClientSecrets("biohop")) != 0 {
		t.Error("a media client became a store client")
	}
}
