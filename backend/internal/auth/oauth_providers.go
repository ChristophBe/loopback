package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/endpoints"
)

const (
	providerTimeout = 10 * time.Second
	maxProviderBody = 1 << 20
)

// LoadOAuthConfig reads OAUTH_BASE_URL (default http://localhost:8080/api/v1)
// and the OAUTH_GITHUB_* and OAUTH_GOOGLE_* client id and secret pairs with
// getenv. A provider without both values is not configured.
func LoadOAuthConfig(getenv func(string) string) OAuthConfig {
	cfg := OAuthConfig{BaseURL: getenv("OAUTH_BASE_URL"), Providers: map[string]Provider{}}
	if cfg.BaseURL == "" {
		cfg.BaseURL = "http://localhost:8080/api/v1"
	}
	if id, secret := getenv("OAUTH_GITHUB_CLIENT_ID"), getenv("OAUTH_GITHUB_CLIENT_SECRET"); id != "" && secret != "" {
		cfg.Providers["github"] = &httpProvider{
			oauth:   oauth2.Config{ClientID: id, ClientSecret: secret, Endpoint: endpoints.GitHub, Scopes: []string{"read:user", "user:email"}},
			profile: githubProfile,
		}
	}
	if id, secret := getenv("OAUTH_GOOGLE_CLIENT_ID"), getenv("OAUTH_GOOGLE_CLIENT_SECRET"); id != "" && secret != "" {
		cfg.Providers["google"] = &httpProvider{
			oauth:   oauth2.Config{ClientID: id, ClientSecret: secret, Endpoint: endpoints.Google, Scopes: []string{"openid", "email"}},
			profile: googleProfile,
		}
	}
	return cfg
}

// httpProvider is a Provider backed by the oauth2 package.
type httpProvider struct {
	oauth   oauth2.Config
	profile func(ctx context.Context, client *http.Client) (Profile, error)
}

func (p *httpProvider) AuthURL(state, redirectURL string) string {
	cfg := p.oauth
	cfg.RedirectURL = redirectURL
	return cfg.AuthCodeURL(state)
}

func (p *httpProvider) Exchange(ctx context.Context, code, redirectURL string) (Profile, error) {
	cfg := p.oauth
	cfg.RedirectURL = redirectURL
	ctx, cancel := context.WithTimeout(ctx, providerTimeout)
	defer cancel()
	token, err := cfg.Exchange(ctx, code)
	var rejected *oauth2.RetrieveError
	if errors.As(err, &rejected) && rejected.Response != nil && rejected.Response.StatusCode < 500 {
		return Profile{}, fmt.Errorf("%w: %v", ErrOAuthExchange, err)
	}
	if err != nil {
		return Profile{}, err
	}
	return p.profile(ctx, cfg.Client(ctx, token))
}

func getJSON(ctx context.Context, client *http.Client, url string, dst any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("GET %s: %s", url, resp.Status)
	}
	return json.NewDecoder(io.LimitReader(resp.Body, maxProviderBody)).Decode(dst)
}

func githubProfile(ctx context.Context, client *http.Client) (Profile, error) {
	var account struct {
		ID int64 `json:"id"`
	}
	if err := getJSON(ctx, client, "https://api.github.com/user", &account); err != nil {
		return Profile{}, err
	}
	var emails []struct {
		Email    string `json:"email"`
		Primary  bool   `json:"primary"`
		Verified bool   `json:"verified"`
	}
	if err := getJSON(ctx, client, "https://api.github.com/user/emails", &emails); err != nil {
		return Profile{}, err
	}
	profile := Profile{ID: strconv.FormatInt(account.ID, 10)}
	for _, e := range emails {
		if e.Primary && e.Verified {
			profile.Email, profile.EmailVerified = e.Email, true
			break
		}
	}
	return profile, nil
}

func googleProfile(ctx context.Context, client *http.Client) (Profile, error) {
	var info struct {
		Sub           string `json:"sub"`
		Email         string `json:"email"`
		EmailVerified bool   `json:"email_verified"`
	}
	if err := getJSON(ctx, client, "https://openidconnect.googleapis.com/v1/userinfo", &info); err != nil {
		return Profile{}, err
	}
	return Profile{ID: info.Sub, Email: info.Email, EmailVerified: info.EmailVerified}, nil
}
