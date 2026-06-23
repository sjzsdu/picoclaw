package qq

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/tencent-connect/botgo/constant"
	"github.com/tencent-connect/botgo/token"
	"golang.org/x/oauth2"
)

const (
	qqTokenEndpoint         = "/app/getAppAccessToken"
	defaultQQTokenExpiresIn = int64(7200)
)

var qqTokenDomain = constant.TokenDomain

type qqTokenSource struct {
	appID     string
	appSecret string
	client    *http.Client
	now       func() time.Time

	mu    sync.Mutex
	token *oauth2.Token
}

type qqTokenRequest struct {
	AppID        string `json:"appId"`
	ClientSecret string `json:"clientSecret"`
}

type qqTokenResponse struct {
	Code        int    `json:"code"`
	Message     string `json:"message"`
	AccessToken string `json:"access_token"`
	ExpiresIn   int64  `json:"expires_in"`
}

func newQQTokenSource(appID, appSecret string, client *http.Client) oauth2.TokenSource {
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	return &qqTokenSource{
		appID:     appID,
		appSecret: appSecret,
		client:    client,
		now:       time.Now,
	}
}

func (s *qqTokenSource) Token() (*oauth2.Token, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.token != nil && s.token.Valid() {
		return s.token, nil
	}

	tk, err := s.fetchToken()
	if err != nil {
		return nil, err
	}
	s.token = tk
	return tk, nil
}

func (s *qqTokenSource) fetchToken() (*oauth2.Token, error) {
	reqBody, err := json.Marshal(qqTokenRequest{
		AppID:        s.appID,
		ClientSecret: s.appSecret,
	})
	if err != nil {
		return nil, fmt.Errorf("marshal QQ token request: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, qqTokenDomain+qqTokenEndpoint, bytes.NewReader(reqBody))
	if err != nil {
		return nil, fmt.Errorf("build QQ token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request QQ token: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read QQ token response: %w", err)
	}

	var payload qqTokenResponse
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("decode QQ token response: %w", err)
	}
	if payload.Code != 0 {
		return nil, fmt.Errorf("QQ token API error %d: %s", payload.Code, payload.Message)
	}
	if payload.AccessToken == "" {
		return nil, fmt.Errorf("QQ token response missing access_token")
	}
	if payload.ExpiresIn <= 0 {
		payload.ExpiresIn = defaultQQTokenExpiresIn
	}

	now := s.now()
	return &oauth2.Token{
		AccessToken: payload.AccessToken,
		TokenType:   token.TypeQQBot,
		Expiry:      now.Add(time.Duration(payload.ExpiresIn) * time.Second),
		ExpiresIn:   payload.ExpiresIn,
	}, nil
}

func (r *qqTokenResponse) UnmarshalJSON(data []byte) error {
	type rawQQTokenResponse struct {
		Code        int             `json:"code"`
		Message     string          `json:"message"`
		AccessToken string          `json:"access_token"`
		ExpiresIn   json.RawMessage `json:"expires_in"`
	}

	var raw rawQQTokenResponse
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	r.Code = raw.Code
	r.Message = raw.Message
	r.AccessToken = raw.AccessToken

	expiresIn, err := parseQQExpiresIn(raw.ExpiresIn)
	if err != nil {
		return err
	}
	r.ExpiresIn = expiresIn
	return nil
}

func parseQQExpiresIn(raw json.RawMessage) (int64, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return 0, nil
	}

	var asString string
	if err := json.Unmarshal(raw, &asString); err == nil {
		asString = strings.TrimSpace(asString)
		if asString == "" {
			return 0, nil
		}
		value, err := strconv.ParseInt(asString, 10, 64)
		if err != nil {
			return 0, fmt.Errorf("parse expires_in string %q: %w", asString, err)
		}
		return value, nil
	}

	var asNumber int64
	if err := json.Unmarshal(raw, &asNumber); err == nil {
		return asNumber, nil
	}

	return 0, fmt.Errorf("unsupported expires_in value: %s", string(raw))
}
