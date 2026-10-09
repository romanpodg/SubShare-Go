package main

import (
	"github.com/romanpodg/SubShare-Go/internal/delivery"
	"net/http"
	"strings"
	"time"
)

type subscriptionTemplate struct {
	ID        int64     `json:"id"`
	Slug      string    `json:"slug"`
	Name      string    `json:"name"`
	Format    string    `json:"format"`
	Content   string    `json:"content"`
	Enabled   bool      `json:"enabled"`
	IsSystem  bool      `json:"is_system"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type responseRuleCondition struct {
	HeaderName    string `json:"headerName"`
	Operator      string `json:"operator"`
	Value         string `json:"value"`
	CaseSensitive bool   `json:"caseSensitive"`
}

type responseHeader struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type responseRule struct {
	ID           int64                   `json:"id"`
	Name         string                  `json:"name"`
	Description  string                  `json:"description"`
	Enabled      bool                    `json:"enabled"`
	Priority     int                     `json:"priority"`
	Operator     string                  `json:"operator"`
	Conditions   []responseRuleCondition `json:"conditions"`
	ResponseType string                  `json:"response_type"`
	TemplateID   *int64                  `json:"template_id"`
	Headers      []responseHeader        `json:"headers"`
	IsSystem     bool                    `json:"is_system"`
	CreatedAt    time.Time               `json:"created_at"`
	UpdatedAt    time.Time               `json:"updated_at"`
}

type templateInput struct {
	Name    string `json:"name"`
	Format  string `json:"format"`
	Content string `json:"content"`
	Enabled bool   `json:"enabled"`
}

type responseRuleInput struct {
	Name         string                  `json:"name"`
	Description  string                  `json:"description"`
	Enabled      bool                    `json:"enabled"`
	Priority     int                     `json:"priority"`
	Operator     string                  `json:"operator"`
	Conditions   []responseRuleCondition `json:"conditions"`
	ResponseType string                  `json:"response_type"`
	TemplateID   *int64                  `json:"template_id"`
	Headers      []responseHeader        `json:"headers"`
}

func (a *App) matchSubscriptionResponseRule(r *http.Request) (*responseRule, error) {
	rules, err := a.listResponseRules()
	if err != nil {
		return nil, err
	}
	return firstMatchingResponseRule(rules, r.Header), nil
}

func applyTemplateContent(content, subscriptionBody, title string) string {
	if strings.TrimSpace(content) == "" {
		return subscriptionBody
	}
	result := strings.ReplaceAll(content, "{{subscription}}", subscriptionBody)
	result = strings.ReplaceAll(result, "{{title}}", title)
	return result
}

func renderMihomoSubscription(raw string) (string, error) {
	generated, err := delivery.RenderMihomo(delivery.SyntheticEntries(raw))
	if err != nil {
		return "", err
	}
	return generated.Body, nil
}

func renderSingBoxSubscription(raw string) (string, error) {
	generated, err := delivery.RenderSingBox(delivery.SyntheticEntries(raw))
	if err != nil {
		return "", err
	}
	return generated.Body, nil
}

func applyRuleHeaders(w http.ResponseWriter, headers []responseHeader) {
	for _, header := range headers {
		if safeCustomResponseHeader(header.Key, header.Value) {
			w.Header().Set(header.Key, header.Value)
		}
	}
}

func (a *App) incrementSubscriptionMetric(responseType, resultStatus string) {
	responseType = strings.TrimSpace(responseType)
	resultStatus = strings.TrimSpace(resultStatus)
	if responseType == "" {
		responseType = "unknown"
	}
	if resultStatus == "" {
		resultStatus = "unknown"
	}
	_, _ = a.db.Exec(`
		INSERT INTO subscription_metrics(day, response_type, result_status, requests)
		VALUES(date('now'), ?, ?, 1)
		ON CONFLICT(day, response_type, result_status)
		DO UPDATE SET requests = requests + 1
	`, responseType, resultStatus)
}
