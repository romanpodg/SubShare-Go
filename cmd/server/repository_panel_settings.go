package main

import (
	"database/sql"
	"strings"

	"github.com/romanpodg/SubShare-Go/internal/model"
)

func (a *App) getPanelSettings() (model.PanelSettings, error) {
	var panelTitle, logoData, faviconData sql.NullString
	var pageTitleAdmin, pageTitleAdminLogin, pageTitleSubscription sql.NullString
	var subscriptionPageConfig sql.NullString

	err := a.db.QueryRow(
		`SELECT panel_title, logo_data, favicon_data, page_title_admin, page_title_admin_login, page_title_subscription, subscription_page_config
		 FROM panel_settings WHERE id = 1`,
	).Scan(&panelTitle, &logoData, &faviconData, &pageTitleAdmin, &pageTitleAdminLogin, &pageTitleSubscription, &subscriptionPageConfig)
	if err != nil {
		return model.PanelSettings{}, err
	}

	s := model.PanelSettings{
		PanelTitle:             strings.TrimSpace(panelTitle.String),
		LogoDataURL:            logoData.String,
		FaviconDataURL:         faviconData.String,
		PageTitleAdmin:         strings.TrimSpace(pageTitleAdmin.String),
		PageTitleAdminLogin:    strings.TrimSpace(pageTitleAdminLogin.String),
		PageTitleSubscription:  strings.TrimSpace(pageTitleSubscription.String),
		SubscriptionPageConfig: subscriptionPageConfig.String,
	}
	if s.PanelTitle == "" {
		s.PanelTitle = "SubShare"
	}
	if s.PageTitleAdmin == "" {
		s.PageTitleAdmin = "Панель управления — SubShare"
	}
	if s.PageTitleAdminLogin == "" {
		s.PageTitleAdminLogin = "Вход — SubShare"
	}
	if s.PageTitleSubscription == "" {
		s.PageTitleSubscription = "VPN-подписка — SubShare"
	}
	return s, nil
}

func (a *App) updatePanelSettings(s model.PanelSettings) error {
	_, err := a.db.Exec(
		`UPDATE panel_settings SET
			panel_title = ?,
			logo_data = ?,
			favicon_data = ?,
			page_title_admin = ?,
			page_title_admin_login = ?,
			page_title_subscription = ?,
			updated_at = CURRENT_TIMESTAMP
		 WHERE id = 1`,
		s.PanelTitle, s.LogoDataURL, s.FaviconDataURL,
		s.PageTitleAdmin, s.PageTitleAdminLogin, s.PageTitleSubscription,
	)
	return err
}

func (a *App) updateSubscriptionPageConfig(configJSON string) error {
	_, err := a.db.Exec(
		`UPDATE panel_settings SET
			subscription_page_config = ?,
			updated_at = CURRENT_TIMESTAMP
		 WHERE id = 1`,
		strings.TrimSpace(configJSON),
	)
	return err
}
