package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strings"

	"github.com/romanpodg/SubShare-Go/internal/model"
)

const (
	routingDeliveryModeDisabled = "disabled"
	routingDeliveryModeAdd      = "add"
	routingDeliveryModeOnAdd    = "onadd"
	happRoutingOffURL           = "happ://routing/off"
)

var routingStringFields = map[string]struct{}{
	"Name": {}, "RouteOrder": {}, "DomainStrategy": {}, "RemoteDNSType": {},
	"RemoteDNSDomain": {}, "RemoteDNSIP": {}, "DomesticDNSType": {},
	"DomesticDNSDomain": {}, "DomesticDNSIP": {}, "Geoipurl": {},
	"Geositeurl": {}, "LastUpdated": {},
}

var routingBooleanFields = map[string]struct{}{
	"GlobalProxy": {}, "FakeDNS": {}, "UseChunkFiles": {},
}

var routingStringListFields = map[string]struct{}{
	"DirectSites": {}, "DirectIp": {}, "ProxySites": {}, "ProxyIp": {},
	"BlockSites": {}, "BlockIp": {},
}

func validRoutingDeliveryMode(mode string) bool {
	switch strings.TrimSpace(mode) {
	case routingDeliveryModeDisabled, routingDeliveryModeAdd, routingDeliveryModeOnAdd:
		return true
	default:
		return false
	}
}

func validateHappRoutingConfig(configJSON string, requireName bool) error {
	configJSON = strings.TrimSpace(configJSON)
	if configJSON == "" {
		if requireName {
			return fmt.Errorf("config_json is required for automatic routing delivery")
		}
		return nil
	}

	var fields map[string]json.RawMessage
	if err := json.Unmarshal([]byte(configJSON), &fields); err != nil || fields == nil {
		return fmt.Errorf("config_json must be a valid JSON object")
	}
	for name, raw := range fields {
		if err := validateHappRoutingField(name, raw); err != nil {
			return err
		}
	}

	if requireName {
		var name string
		raw, ok := fields["Name"]
		if !ok || json.Unmarshal(raw, &name) != nil || strings.TrimSpace(name) == "" {
			return fmt.Errorf("config_json field Name is required for automatic routing delivery")
		}
	}
	return nil
}

// validateHappRoutingField checks one known config_json field against its
// expected JSON type. Unknown fields are accepted.
func validateHappRoutingField(name string, raw json.RawMessage) error {
	isNull := string(raw) == "null"
	if _, ok := routingStringFields[name]; ok {
		var value string
		if isNull || json.Unmarshal(raw, &value) != nil {
			return fmt.Errorf("config_json field %s must be a string", name)
		}
		return nil
	}
	if _, ok := routingBooleanFields[name]; ok {
		var value bool
		if isNull || json.Unmarshal(raw, &value) != nil {
			return fmt.Errorf("config_json field %s must be a boolean", name)
		}
		return nil
	}
	if _, ok := routingStringListFields[name]; ok {
		var value []string
		if isNull || json.Unmarshal(raw, &value) != nil {
			return fmt.Errorf("config_json field %s must be an array of strings", name)
		}
		return nil
	}
	if name == "DnsHosts" {
		var value map[string]string
		if err := json.Unmarshal(raw, &value); err != nil || value == nil {
			return fmt.Errorf("config_json field DnsHosts must be an object with string values")
		}
	}
	return nil
}

func buildHappRoutingLink(configJSON, mode string) (string, error) {
	mode = strings.TrimSpace(mode)
	if mode != routingDeliveryModeAdd && mode != routingDeliveryModeOnAdd {
		return "", fmt.Errorf("unsupported routing delivery mode %q", mode)
	}
	if err := validateHappRoutingConfig(configJSON, true); err != nil {
		return "", err
	}
	payload := base64.StdEncoding.EncodeToString([]byte(strings.TrimSpace(configJSON)))
	// QueryEscape matches encodeURIComponent for the Base64 alphabet used by
	// existing Happ links (%2B, %2F, %3D) without changing the payload bytes.
	return "happ://routing/" + mode + "/" + url.QueryEscape(payload), nil
}

func routingSettingsWithLinks(settings model.RoutingSettings) model.RoutingSettings {
	settings.AddURL = ""
	settings.OnAddURL = ""
	settings.OffURL = happRoutingOffURL
	if settings.ConfigJSON == "" {
		return settings
	}
	if link, err := buildHappRoutingLink(settings.ConfigJSON, routingDeliveryModeAdd); err == nil {
		settings.AddURL = link
	}
	if link, err := buildHappRoutingLink(settings.ConfigJSON, routingDeliveryModeOnAdd); err == nil {
		settings.OnAddURL = link
	}
	return settings
}

func (a *App) applyHappRoutingHeader(header http.Header) {
	// Routing is canonical metadata. Clear any value set by a legacy path before
	// deciding whether this installation manages routing automatically.
	header.Del("routing")
	settings, err := a.getRoutingSettings()
	if err != nil {
		log.Printf("routing delivery omitted: failed to load routing settings: %v", err)
		return
	}
	if strings.TrimSpace(settings.DeliveryMode) == routingDeliveryModeDisabled {
		return
	}
	link, err := buildHappRoutingLink(settings.ConfigJSON, settings.DeliveryMode)
	if err != nil {
		log.Printf("routing delivery omitted: invalid stored routing settings: %v", err)
		return
	}
	header.Set("routing", link)
}
