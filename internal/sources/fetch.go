package sources

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Fetch downloads and parses a subscription. client may be nil, in which case
// the SSRF-guarded default is used; tests inject their own.
func Fetch(ctx context.Context, client *http.Client, sourceURL string, hwidProfile HWIDProfile, fingerprintKeys [][]byte) (ParseResult, error) {
	finalURL, err := ValidateURL(sourceURL)
	if err != nil {
		return ParseResult{}, err
	}

	if ctx == nil {
		ctx = context.Background()
	}
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, finalURL, nil)
	if err != nil {
		return ParseResult{}, fmt.Errorf("failed to prepare source request")
	}
	applyExternalHWIDHeaders(req, hwidProfile)
	req.Header.Set("Accept", "application/json,text/plain,*/*")

	if client == nil {
		client = NewHTTPClient()
	}
	resp, err := client.Do(req)
	if err != nil {
		// net/http errors commonly embed the complete URL. Subscription URLs
		// frequently contain access tokens, so never persist or return them.
		return ParseResult{}, fmt.Errorf("failed to fetch source")
	}
	defer resp.Body.Close()

	bodyBytes, err := readSubscriptionResponse(resp)
	if err != nil {
		return ParseResult{}, err
	}
	parsed, err := parseFetchedBody(bodyBytes, fingerprintKeys)
	if err != nil {
		return ParseResult{}, err
	}
	metadata, titleWarnings := metadataFromResponse(resp)
	parsed.Metadata = metadata
	parsed.Warnings = append(parsed.Warnings, titleWarnings...)
	return parsed, nil
}

// readSubscriptionResponse enforces the size cap, status range and non-empty
// body before any parsing happens.
func readSubscriptionResponse(resp *http.Response) ([]byte, error) {
	bodyBytes, err := io.ReadAll(io.LimitReader(resp.Body, MaxBodyBytes+1))
	if err != nil {
		return nil, fmt.Errorf("failed to read source response: %w", err)
	}
	if len(bodyBytes) > MaxBodyBytes {
		return nil, fmt.Errorf("source response is too large (max 10 MB)")
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusBadRequest {
		return nil, fmt.Errorf("source returned HTTP %d", resp.StatusCode)
	}
	if len(bytes.TrimSpace(bodyBytes)) != 0 {
		return bodyBytes, nil
	}
	if resp.Header.Get("profile-title") != "" || resp.Header.Get("announce") != "" {
		return nil, fmt.Errorf("source returned subscription headers but empty body")
	}
	return nil, fmt.Errorf("subscription body is empty")
}

// parseFetchedBody parses the body and insists on at least one supported key.
func parseFetchedBody(bodyBytes []byte, fingerprintKeys [][]byte) (ParseResult, error) {
	parsed, err := ParseBody(string(bodyBytes), fingerprintKeys)
	if err != nil {
		return ParseResult{}, err
	}
	if len(parsed.Keys) == 0 {
		return ParseResult{}, NoSupportedKeysError(parsed)
	}
	if len(parsed.Keys) > MaxImportItems {
		return ParseResult{}, fmt.Errorf("too many keys in source response (max %d)", MaxImportItems)
	}
	return parsed, nil
}

// metadataFromResponse reads the subscription headers; the second value is
// the warning emitted when the remote title had to be shortened.
func metadataFromResponse(resp *http.Response) (Metadata, []string) {
	remoteTitle, titleWarnings := NormalizeRemoteProfileTitle(DecodeHeaderValue(resp.Header.Get("profile-title")))
	return Metadata{
		Title:           remoteTitle,
		RefreshHours:    parsePositiveInt(resp.Header.Get("profile-update-interval")),
		SupportURL:      strings.TrimSpace(resp.Header.Get("support-url")),
		WebPageURL:      strings.TrimSpace(resp.Header.Get("profile-web-page-url")),
		Announce:        DecodeHeaderValue(resp.Header.Get("announce")),
		ContentType:     strings.TrimSpace(resp.Header.Get("content-type")),
		ContentDisp:     strings.TrimSpace(resp.Header.Get("content-disposition")),
		SourceFinalURL:  strings.TrimSpace(resp.Request.URL.String()),
		HTTPStatusCode:  resp.StatusCode,
		HTTPStatusLabel: strings.TrimSpace(resp.Status),
	}, titleWarnings
}
