package apns

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/snowskeleton/igg-server/internal/config"
)

const (
	developmentEndpoint = "https://api.sandbox.push.apple.com"
	productionEndpoint  = "https://api.push.apple.com"
	jwtCacheDuration    = 50 * time.Minute
)

type Client struct {
	key      *ecdsa.PrivateKey
	keyID    string
	teamID   string
	bundleID string
	endpoint string
	client   *http.Client

	mu       sync.Mutex
	jwtToken string
	jwtExp   time.Time
}

func NewClient(cfg *config.Config) (*Client, error) {
	if cfg.APNsKeyID == "" || cfg.APNsTeamID == "" {
		return nil, nil
	}

	var keyData []byte
	var err error

	if cfg.APNsKeyContent != "" {
		keyData = []byte(cfg.APNsKeyContent)
	} else if cfg.APNsKeyPath != "" {
		keyData, err = os.ReadFile(cfg.APNsKeyPath)
		if err != nil {
			return nil, fmt.Errorf("read APNs key file: %w", err)
		}
	} else {
		return nil, fmt.Errorf("APNS_KEY_PATH or APNS_KEY_CONTENT is required when APNS_KEY_ID is set")
	}

	key, err := parseP8Key(keyData)
	if err != nil {
		return nil, fmt.Errorf("parse APNs key: %w", err)
	}

	endpoint := developmentEndpoint
	if cfg.APNsProduction {
		endpoint = productionEndpoint
	}

	return &Client{
		key:      key,
		keyID:    cfg.APNsKeyID,
		teamID:   cfg.APNsTeamID,
		bundleID: cfg.APNsBundleID,
		endpoint: endpoint,
		client:   &http.Client{Timeout: 10 * time.Second},
	}, nil
}

func parseP8Key(data []byte) (*ecdsa.PrivateKey, error) {
	block, _ := pem.Decode(data)
	if block == nil {
		return nil, fmt.Errorf("failed to decode PEM block")
	}

	key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parse PKCS8 key: %w", err)
	}

	ecKey, ok := key.(*ecdsa.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("key is not ECDSA")
	}
	return ecKey, nil
}

func (c *Client) getJWT() (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.jwtToken != "" && time.Now().Before(c.jwtExp) {
		return c.jwtToken, nil
	}

	now := time.Now()
	token := jwt.NewWithClaims(jwt.SigningMethodES256, jwt.MapClaims{
		"iss": c.teamID,
		"iat": now.Unix(),
	})
	token.Header["kid"] = c.keyID

	signed, err := token.SignedString(c.key)
	if err != nil {
		return "", fmt.Errorf("sign JWT: %w", err)
	}

	c.jwtToken = signed
	c.jwtExp = now.Add(jwtCacheDuration)
	return signed, nil
}

// SendBackground sends a silent content-available push (priority 5).
// Returns true if the token should be removed (device unregistered).
func (c *Client) SendBackground(token string) (removeToken bool) {
	payload := map[string]any{
		"aps": map[string]any{
			"content-available": 1,
		},
	}
	return c.send(token, payload, "5", "background")
}

// SendAlert sends a visible notification with banner and sound (priority 10).
// Returns true if the token should be removed (device unregistered).
func (c *Client) SendAlert(token, title, body string) (removeToken bool) {
	payload := map[string]any{
		"aps": map[string]any{
			"alert": map[string]string{
				"title": title,
				"body":  body,
			},
			"sound":             "default",
			"content-available": 1,
		},
	}
	return c.send(token, payload, "10", "alert")
}

func (c *Client) send(token string, payload any, priority, pushType string) (removeToken bool) {
	jwtStr, err := c.getJWT()
	if err != nil {
		log.Printf("apns: get JWT: %v", err)
		return false
	}

	body, err := json.Marshal(payload)
	if err != nil {
		log.Printf("apns: marshal payload: %v", err)
		return false
	}

	url := fmt.Sprintf("%s/3/device/%s", c.endpoint, token)
	req, err := http.NewRequest("POST", url, bytes.NewReader(body))
	if err != nil {
		log.Printf("apns: create request: %v", err)
		return false
	}

	req.Header.Set("authorization", "bearer "+jwtStr)
	req.Header.Set("apns-topic", c.bundleID)
	req.Header.Set("apns-push-type", pushType)
	req.Header.Set("apns-priority", priority)

	resp, err := c.client.Do(req)
	if err != nil {
		log.Printf("apns: send push: %v", err)
		return false
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	switch resp.StatusCode {
	case 200:
		return false
	case 410:
		return true
	case 400:
		var errResp struct {
			Reason string `json:"reason"`
		}
		json.Unmarshal(respBody, &errResp)
		log.Printf("apns: bad request for token %.8s...: %s", token, errResp.Reason)
		if errResp.Reason == "BadDeviceToken" || errResp.Reason == "Unregistered" {
			return true
		}
		return false
	default:
		log.Printf("apns: unexpected status %d for token %.8s...: %s", resp.StatusCode, token, string(respBody))
		return false
	}
}
