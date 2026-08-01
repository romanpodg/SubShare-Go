package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

type happEncryptRequest struct {
	URL string `json:"url"`
}

type happEncryptResponse struct {
	EncryptedLink string `json:"encrypted_link"`
}

func (a *App) encryptSubscriptionURL(rawURL string) (string, error) {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return "", fmt.Errorf("empty subscription url")
	}

	endpoint := strings.TrimSpace(a.happCryptoAPIURL)
	if endpoint == "" {
		return "", fmt.Errorf("happ crypto api url is not configured")
	}

	payload, err := json.Marshal(happEncryptRequest{URL: rawURL})
	if err != nil {
		return "", err
	}

	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return "", fmt.Errorf("prepare Happ crypto API request")
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("call Happ crypto API")
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("happ crypto api returned status %d", resp.StatusCode)
	}

	var result happEncryptResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}

	encrypted := strings.TrimSpace(result.EncryptedLink)
	if encrypted == "" {
		return "", fmt.Errorf("happ crypto api returned empty encrypted_link")
	}

	return encrypted, nil
}
