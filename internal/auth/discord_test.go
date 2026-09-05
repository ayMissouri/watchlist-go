package auth

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"golang.org/x/oauth2"
)

func TestFetchDiscordUserHasAccess(t *testing.T) {
	for _, tc := range []struct {
		guilds string
		want   bool
	}{
		{`[{"id":"1"},{"id":"626739861025587200"}]`, true},
		{`[{"id":"1545627922532933734"}]`, true},
		{`[{"id":"1"},{"id":"2"}]`, false},
		{`[]`, false},
	} {
		mux := http.NewServeMux()
		mux.HandleFunc("/oauth2/token", func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = fmt.Fprint(w, `{"access_token":"tok","token_type":"bearer"}`)
		})
		mux.HandleFunc("/users/@me", func(w http.ResponseWriter, _ *http.Request) {
			_, _ = fmt.Fprint(w, `{"id":"42","username":"u","avatar":"a"}`)
		})
		mux.HandleFunc("/users/@me/guilds", func(w http.ResponseWriter, r *http.Request) {
			if r.Header.Get("Authorization") != "Bearer tok" {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			_, _ = fmt.Fprint(w, tc.guilds)
		})
		srv := httptest.NewServer(mux)

		OAuthConfig = &oauth2.Config{
			ClientID: "id", ClientSecret: "secret",
			Endpoint: oauth2.Endpoint{TokenURL: srv.URL + "/oauth2/token"},
		}
		discordAPI = srv.URL

		u, err := FetchDiscordUser(t.Context(), "code")
		srv.Close()
		if err != nil {
			t.Fatalf("guilds=%s: %v", tc.guilds, err)
		}
		if u.ID != "42" || u.HasAccess != tc.want {
			t.Errorf("guilds=%s: got id=%q has_access=%v, want has_access=%v", tc.guilds, u.ID, u.HasAccess, tc.want)
		}
	}
}
