package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/ravener/discord-oauth2"
	"golang.org/x/oauth2"
)

// OAuthConfig holds the Discord OAuth2 configuration.
var OAuthConfig *oauth2.Config

func InitDiscord() {
	OAuthConfig = &oauth2.Config{
		ClientID:     os.Getenv("DISCORD_CLIENT_ID"),
		ClientSecret: os.Getenv("DISCORD_CLIENT_SECRET"),
		RedirectURL:  os.Getenv("DISCORD_REDIRECT_URL"),
		Scopes:       []string{discord.ScopeIdentify},
		Endpoint:     discord.Endpoint,
	}
}

// DiscordUser is the relevant part of Discord's /users/@me response.
type DiscordUser struct {
	ID       string `json:"id"`
	Username string `json:"username"`
	Avatar   string `json:"avatar"`
}

// FetchDiscordUser exchanges an OAuth2 code for a token,
// then calls Discord's API to get the authenticated user's profile.
func FetchDiscordUser(ctx context.Context, code string) (*DiscordUser, error) {
	token, err := OAuthConfig.Exchange(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("token exchange: %w", err)
	}

	// OAuthConfig.Client gives us a http.Client that automatically
	// attaches the Bearer token to every request
	client := OAuthConfig.Client(ctx, token)

	resp, err := client.Get("https://discord.com/api/users/@me")
	if err != nil {
		return nil, fmt.Errorf("discord api request: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("discord api returned %d: %s", resp.StatusCode, body)
	}

	var u DiscordUser
	if err := json.NewDecoder(resp.Body).Decode(&u); err != nil {
		return nil, fmt.Errorf("decode discord user: %w", err)
	}

	return &u, nil
}
