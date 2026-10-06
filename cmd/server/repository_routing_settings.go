package main

import (
	"database/sql"
	"errors"
	"strings"

	"github.com/romanpodg/SubShare-Go/internal/model"
)

func (a *App) getRoutingSettings() (model.RoutingSettings, error) {
	var configJSON, deliveryMode sql.NullString
	if err := a.db.QueryRow(`SELECT config_json, delivery_mode FROM routing_settings WHERE id = 1`).Scan(&configJSON, &deliveryMode); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return model.RoutingSettings{ConfigJSON: "", DeliveryMode: routingDeliveryModeDisabled}, nil
		}
		return model.RoutingSettings{}, err
	}

	return model.RoutingSettings{
		ConfigJSON:   strings.TrimSpace(configJSON.String),
		DeliveryMode: strings.TrimSpace(deliveryMode.String),
	}, nil
}

func (a *App) updateRoutingSettings(s model.RoutingSettings) error {
	_, err := a.db.Exec(
		`INSERT INTO routing_settings(id, config_json, delivery_mode, updated_at)
		 VALUES(1, ?, ?, CURRENT_TIMESTAMP)
		 ON CONFLICT(id) DO UPDATE SET
		   config_json = excluded.config_json,
		   delivery_mode = excluded.delivery_mode,
		   updated_at = CURRENT_TIMESTAMP`,
		strings.TrimSpace(s.ConfigJSON), strings.TrimSpace(s.DeliveryMode),
	)
	return err
}
