package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"slices"
	"strings"

	"github.com/ravener/discord-oauth2"
	"golang.org/x/oauth2"
)

// OAuthConfig holds the Discord OAuth2 configuration.
var OAuthConfig *oauth2.Config
var accessGuildIDs = []string{"1545627922532933734", "626739861025587200"}
var discordAPI = "https://discord.com/api"

func InitDiscord() {
	OAuthConfig = &oauth2.Config{
		ClientID:     os.Getenv("DISCORD_CLIENT_ID"),
		ClientSecret: os.Getenv("DISCORD_CLIENT_SECRET"),
		RedirectURL:  os.Getenv("DISCORD_REDIRECT_URL"),
		Scopes:       []string{discord.ScopeIdentify, discord.ScopeGuilds},
		Endpoint:     discord.Endpoint,
	}
}

// DiscordUser is the relevant part of Discord's /users/@me response.
type DiscordUser struct {
	ID        string `json:"id"`
	Username  string `json:"username"`
	Avatar    string `json:"avatar"`
	HasAccess bool   `json:"-"`
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

	var u DiscordUser
	if err := getJSON(client, discordAPI+"/users/@me", &u); err != nil {
		return nil, err
	}

	type guild struct {
		ID string `json:"id"`
	}
	var guilds []guild
	if err := getJSON(client, discordAPI+"/users/@me/guilds", &guilds); err != nil {
		return nil, err
	}
	u.HasAccess = slices.ContainsFunc(guilds, func(g guild) bool {
		return slices.Contains(accessGuildIDs, g.ID)
	})

	return &u, nil
}

func getJSON(client *http.Client, url string, dst any) error {
	resp, err := client.Get(url)
	if err != nil {
		return fmt.Errorf("discord api request: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("discord api returned %d: %s", resp.StatusCode, body)
	}

	if err := json.NewDecoder(resp.Body).Decode(dst); err != nil {
		return fmt.Errorf("decode discord response: %w", err)
	}
	return nil
}

func AvatarURL(userID, avatar string) string {
	if avatar == "" {
		return ""
	}
	if strings.HasPrefix(avatar, "http://") || strings.HasPrefix(avatar, "https://") {
		return avatar
	}
	return fmt.Sprintf("https://cdn.discordapp.com/avatars/%s/%s.webp", userID, avatar)
}
