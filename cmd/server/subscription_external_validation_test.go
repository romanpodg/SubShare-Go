package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/romanpodg/SubShare-Go/internal/model"
)

const (
	externalValidationRejected        = "official_client_rejected"
	externalValidationVersionMismatch = "official_client_version_mismatch"
	externalValidationFixtureInvalid  = "validation_fixture_invalid"
	externalValidationCleanupFailed   = "validation_cleanup_failed"
)

type externalValidationFixture struct {
	name    string
	content []byte
}

type externalValidatorCommand func(binary, configPath, workDir string) *exec.Cmd

func validateExternalFixture(binary, extension string, content []byte, command externalValidatorCommand) (workspace string, resultErr error) {
	workspace, err := os.MkdirTemp("", "subshare-official-validator-")
	if err != nil {
		return "", errors.New(externalValidationFixtureInvalid)
	}
	defer func() {
		if cleanupErr := os.RemoveAll(workspace); cleanupErr != nil && resultErr == nil {
			resultErr = errors.New(externalValidationCleanupFailed)
		}
	}()

	configPath := filepath.Join(workspace, "config"+extension)
	if err := os.WriteFile(configPath, content, 0o600); err != nil {
		return workspace, errors.New(externalValidationFixtureInvalid)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	cmd := command(binary, configPath, workspace)
	cmd = exec.CommandContext(ctx, cmd.Path, cmd.Args[1:]...)
	cmd.Dir = workspace
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	if err := cmd.Run(); err != nil {
		return workspace, errors.New(externalValidationRejected)
	}
	return workspace, nil
}

func requireExternalBinaryVersion(t *testing.T, envName, expected string, args ...string) string {
	t.Helper()
	binary := strings.TrimSpace(os.Getenv(envName))
	if binary == "" {
		t.Skipf("%s is not set; official client validation skipped", envName)
	}
	if info, err := os.Stat(binary); err != nil || info.IsDir() {
		t.Fatalf("%s: %s", envName, externalValidationFixtureInvalid)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	output, err := exec.CommandContext(ctx, binary, args...).CombinedOutput()
	if err != nil || !strings.Contains(string(output), expected) {
		t.Fatalf("%s: %s", envName, externalValidationVersionMismatch)
	}
	return binary
}

func requireNoGeneratorExclusions(exclusions []generationExclusion) error {
	if len(exclusions) != 0 {
		return errors.New(exclusions[0].Reason)
	}
	return nil
}

func completeMihomoValidationConfig(entries []deliveryEntry) ([]byte, error) {
	generated, err := renderMihomoEntries(entries)
	if err != nil {
		return nil, errors.New(externalValidationFixtureInvalid)
	}
	if exclusionErr := requireNoGeneratorExclusions(generated.Exclusions); exclusionErr != nil {
		return nil, exclusionErr
	}
	templated := applyTemplateContent("{{subscription}}", generated.Body, "Synthetic validation")
	if err := validateGeneratedStructuredBody("mihomo", templated); err != nil {
		return nil, errors.New(externalValidationFixtureInvalid)
	}
	var generatedRoot map[string]any
	if err := json.Unmarshal([]byte(templated), &generatedRoot); err != nil {
		return nil, errors.New(externalValidationFixtureInvalid)
	}
	proxies, ok := generatedRoot["proxies"].([]any)
	if !ok || len(proxies) == 0 {
		return nil, errors.New(externalValidationFixtureInvalid)
	}
	names := make([]string, 0, len(proxies))
	for _, value := range proxies {
		proxy, ok := value.(map[string]any)
		name, nameOK := proxy["name"].(string)
		if !ok || !nameOK || name == "" {
			return nil, errors.New(externalValidationFixtureInvalid)
		}
		names = append(names, name)
	}
	return json.MarshalIndent(map[string]any{
		"mixed-port": 7890,
		"mode":       "rule",
		"log-level":  "silent",
		"proxies":    proxies,
		"proxy-groups": []any{map[string]any{
			"name": "VALIDATION", "type": "select", "proxies": names,
		}},
		"rules": []string{"MATCH,VALIDATION"},
	}, "", "  ")
}

func completeSingBoxValidationConfig(entries []deliveryEntry) ([]byte, error) {
	generated, err := renderSingBoxEntries(entries)
	if err != nil {
		return nil, errors.New(externalValidationFixtureInvalid)
	}
	if exclusionErr := requireNoGeneratorExclusions(generated.Exclusions); exclusionErr != nil {
		return nil, exclusionErr
	}
	templated := applyTemplateContent("{{subscription}}", generated.Body, "Synthetic validation")
	if err := validateGeneratedStructuredBody("sing-box", templated); err != nil {
		return nil, errors.New(externalValidationFixtureInvalid)
	}
	var generatedRoot map[string]any
	if err := json.Unmarshal([]byte(templated), &generatedRoot); err != nil {
		return nil, errors.New(externalValidationFixtureInvalid)
	}
	outbounds, ok := generatedRoot["outbounds"].([]any)
	if !ok || len(outbounds) == 0 {
		return nil, errors.New(externalValidationFixtureInvalid)
	}
	first, ok := outbounds[0].(map[string]any)
	firstTag, tagOK := first["tag"].(string)
	if !ok || !tagOK || firstTag == "" {
		return nil, errors.New(externalValidationFixtureInvalid)
	}
	return json.MarshalIndent(map[string]any{
		"log":       map[string]any{"level": "error"},
		"outbounds": outbounds,
		"route":     map[string]any{"final": firstTag},
	}, "", "  ")
}

func completeXrayValidationConfig(entries []deliveryEntry) ([]byte, error) {
	generated, err := renderXrayEntries(entries)
	if err != nil {
		return nil, errors.New(externalValidationFixtureInvalid)
	}
	if exclusionErr := requireNoGeneratorExclusions(generated.Exclusions); exclusionErr != nil {
		return nil, exclusionErr
	}
	templated := applyTemplateContent("{{subscription}}", generated.Body, "Synthetic validation")
	if err := validateGeneratedStructuredBody("xray-json", templated); err != nil {
		return nil, errors.New(externalValidationFixtureInvalid)
	}
	var documents []map[string]any
	if err := json.Unmarshal([]byte(templated), &documents); err != nil || len(documents) == 0 {
		return nil, errors.New(externalValidationFixtureInvalid)
	}
	outbounds := make([]any, 0, len(documents))
	for _, document := range documents {
		items, ok := document["outbounds"].([]any)
		if !ok || len(items) == 0 {
			return nil, errors.New(externalValidationFixtureInvalid)
		}
		outbounds = append(outbounds, items...)
	}
	return json.MarshalIndent(map[string]any{
		"log":       map[string]any{"loglevel": "none"},
		"outbounds": outbounds,
	}, "", "  ")
}

func validationEntries(raw ...string) []deliveryEntry {
	entries := make([]deliveryEntry, 0, len(raw))
	for index, item := range raw {
		entries = append(entries, deliveryEntry{ID: int64(index + 1), Raw: item, Kind: model.KeyKindReal})
	}
	return entries
}

const (
	validationSS2022              = "ss://2022-blake3-aes-128-gcm:YWJjZGVmZ2hpamtsbW5vcA%3D%3D@ss2022.example:443#SS-2022"
	validationSSV2Ray             = "ss://YWVzLTI1Ni1nY206c3ludGhldGljLXZhbGlkYXRpb24=@ss-plugin.example:8388?plugin=v2ray-plugin%3Bmode%3Dwebsocket%3Bhost%3Dcdn.example%3Bpath%3D%2Fws%3Btls%3Dtrue%3Bmux%3Dtrue#SS-v2ray"
	validationSSV2RaySingBox      = "ss://YWVzLTI1Ni1nY206c3ludGhldGljLXZhbGlkYXRpb24=@ss-plugin.example:8388?plugin=v2ray-plugin%3Bmode%3Dwebsocket%3Bhost%3Dcdn.example%3Bpath%3D%2Fws%3Btls%3Dtrue%3Bmux%3D8#SS-v2ray"
	validationHY2Basic            = "hysteria2://synthetic-auth@hy-basic.example:443?sni=hy-basic.example#HY2-basic"
	validationHY2Ports            = "hy2://synthetic-auth@hy-ports.example:443,5000-5002?sni=hy-ports.example#HY2-ports"
	validationHY2Salamander       = "hysteria2://synthetic-auth@hy-obfs.example:443?sni=hy-obfs.example&obfs=salamander&obfs-password=synthetic-obfs#HY2-salamander"
	validationHY2Gecko            = "hysteria2://synthetic-auth@hy-gecko.example:443?sni=hy-gecko.example&obfs=gecko&obfs-password=synthetic-gecko#HY2-gecko"
	validationHY2IPv6             = "hysteria2://synthetic-auth@[2001:db8::10]:443?sni=ipv6.example#HY2-IPv6"
	validationTUICBasic           = "tuic://44444444-4444-4444-8444-444444444444:synthetic-password@tuic-basic.example:443?sni=tuic-basic.example#TUIC-basic"
	validationTUICAdvancedMihomo  = "tuic://55555555-5555-4555-8555-555555555555:synthetic-password@tuic-advanced.example:443?sni=tuic-advanced.example&alpn=h3%2Chq-29&skip-cert-verify=true&congestion-controller=bbr&udp-relay-mode=quic&zero-rtt=true&heartbeat=10s&request-timeout=8s&fast-open=true&max-open-streams=20&max-udp-relay-packet-size=1500#TUIC-advanced"
	validationTUICAdvancedSingBox = "tuic://55555555-5555-4555-8555-555555555555:synthetic-password@tuic-advanced.example:443?server_name=tuic-advanced.example&alpn=h3%2Chq-29&allow_insecure=true&congestion_control=bbr&udp_over_stream=true&zero_rtt_handshake=true&heartbeat=10s#TUIC-advanced"
)

func buildMihomoExternalFixtures() ([]externalValidationFixture, error) {
	tables := []struct {
		name string
		raw  []string
	}{
		{"vless", []string{externalTestVLESS}},
		{"vmess", []string{externalTestVMess}},
		{"trojan", []string{externalTestTrojan}},
		{"ss-basic", []string{deliverySSClean}},
		{"ss-plugin-obfs", []string{deliverySSPlugin}},
		{"ss-plugin-v2ray", []string{validationSSV2Ray}},
		{"ss-2022", []string{validationSS2022}},
		{"hysteria2-single-port", []string{validationHY2Basic}},
		{"hysteria2-port-expression", []string{validationHY2Ports}},
		{"hysteria2-salamander", []string{validationHY2Salamander}},
		{"hysteria2-gecko", []string{validationHY2Gecko}},
		{"tuic-v5-basic", []string{validationTUICBasic}},
		{"tuic-v5-advanced", []string{validationTUICAdvancedMihomo}},
		{"ipv6", []string{validationHY2IPv6}},
		{"duplicate-display-names", []string{
			"ss://YWVzLTI1Ni1nY206Zmlyc3Qtc3ludGhldGlj@first.example:8388#Duplicate",
			"ss://YWVzLTI1Ni1nY206c2Vjb25kLXN5bnRoZXRpYw@second.example:8388#Duplicate",
		}},
		{"mixed", []string{deliverySSClean, validationHY2Salamander, validationTUICBasic}},
	}
	fixtures := make([]externalValidationFixture, 0, len(tables))
	for _, item := range tables {
		content, err := completeMihomoValidationConfig(validationEntries(item.raw...))
		if err != nil {
			return nil, errors.New(item.name + ":" + err.Error())
		}
		fixtures = append(fixtures, externalValidationFixture{item.name, content})
	}
	return fixtures, nil
}

func buildSingBoxExternalFixtures() ([]externalValidationFixture, error) {
	tables := []struct {
		name string
		raw  []string
	}{
		{"vless", []string{externalTestVLESS}},
		{"vmess", []string{externalTestVMess}},
		{"trojan", []string{externalTestTrojan}},
		{"ss-basic", []string{deliverySSClean}},
		{"ss-plugin-obfs", []string{deliverySSPlugin}},
		{"ss-plugin-v2ray", []string{validationSSV2RaySingBox}},
		{"ss-2022", []string{validationSS2022}},
		{"hysteria2-single-port", []string{validationHY2Basic}},
		{"hysteria2-ordered-ports", []string{validationHY2Ports}},
		{"hysteria2-tls-salamander", []string{validationHY2Salamander}},
		{"tuic-v5-basic", []string{validationTUICBasic}},
		{"tuic-v5-udp-over-stream", []string{validationTUICAdvancedSingBox}},
		{"ipv6", []string{validationHY2IPv6}},
		{"mixed", []string{deliverySSClean, validationHY2Salamander, validationTUICBasic}},
	}
	fixtures := make([]externalValidationFixture, 0, len(tables))
	for _, item := range tables {
		content, err := completeSingBoxValidationConfig(validationEntries(item.raw...))
		if err != nil {
			return nil, errors.New(item.name + ":" + err.Error())
		}
		fixtures = append(fixtures, externalValidationFixture{item.name, content})
	}
	return fixtures, nil
}

func buildXrayExternalFixtures() ([]externalValidationFixture, error) {
	tables := []struct {
		name string
		raw  []string
	}{
		{"vless", []string{externalTestVLESS}},
		{"vmess", []string{externalTestVMess}},
		{"trojan", []string{externalTestTrojan}},
		{"shadowsocks", []string{deliverySSClean}},
		{"shadowsocks-2022", []string{validationSS2022}},
		{"hysteria2-basic", []string{validationHY2Basic}},
		{"hysteria2-tls-auth", []string{"hy2://synthetic-auth@hy-tls.example:443?sni=hy-tls.example#HY2-TLS"}},
		{"hysteria2-certificate-pin", []string{"hy2://synthetic-auth@hy-pin.example:443?sni=hy-pin.example&pinSHA256=abababababababababababababababababababababababababababababababab#HY2-pin"}},
		{"hysteria2-port-hopping", []string{validationHY2Ports}},
		{"hysteria2-salamander", []string{validationHY2Salamander}},
		{"mixed", []string{externalTestVLESS, deliverySSClean, validationHY2Salamander}},
	}
	fixtures := make([]externalValidationFixture, 0, len(tables))
	for _, item := range tables {
		content, err := completeXrayValidationConfig(validationEntries(item.raw...))
		if err != nil {
			return nil, errors.New(item.name + ":" + err.Error())
		}
		fixtures = append(fixtures, externalValidationFixture{item.name, content})
	}
	return fixtures, nil
}

func runExternalFixtureTable(t *testing.T, binary, extension string, fixtures []externalValidationFixture, command externalValidatorCommand) {
	t.Helper()
	for _, fixture := range fixtures {
		t.Run(fixture.name, func(t *testing.T) {
			workspace, err := validateExternalFixture(binary, extension, fixture.content, command)
			if err != nil {
				t.Fatalf("%s: %s", fixture.name, err.Error())
			}
			if _, statErr := os.Stat(workspace); !os.IsNotExist(statErr) {
				t.Fatalf("%s: %s", fixture.name, externalValidationCleanupFailed)
			}
		})
	}
}

func TestOfficialMihomoValidation(t *testing.T) {
	binary := requireExternalBinaryVersion(t, "MIHOMO_BIN", "Mihomo Meta v"+targetMihomoVersion, "-v")
	fixtures, err := buildMihomoExternalFixtures()
	if err != nil {
		t.Fatal(err.Error())
	}
	runExternalFixtureTable(t, binary, ".yaml", fixtures, func(binary, configPath, workDir string) *exec.Cmd {
		return exec.Command(binary, "-t", "-d", workDir, "-f", configPath)
	})
}

func TestOfficialSingBoxValidation(t *testing.T) {
	binary := requireExternalBinaryVersion(t, "SING_BOX_BIN", "sing-box version "+targetSingBoxVersion, "version")
	fixtures, err := buildSingBoxExternalFixtures()
	if err != nil {
		t.Fatal(err.Error())
	}
	runExternalFixtureTable(t, binary, ".json", fixtures, func(binary, configPath, workDir string) *exec.Cmd {
		return exec.Command(binary, "check", "--disable-color", "-D", workDir, "-c", configPath)
	})
}

func TestOfficialXrayMinimumValidation(t *testing.T) {
	binary := requireExternalBinaryVersion(t, "XRAY_26327_BIN", "Xray "+targetXrayMinimumVersion, "version")
	fixtures, err := buildXrayExternalFixtures()
	if err != nil {
		t.Fatal(err.Error())
	}
	runExternalFixtureTable(t, binary, ".json", fixtures, func(binary, configPath, _ string) *exec.Cmd {
		return exec.Command(binary, "run", "-test", "-c", configPath)
	})
}

func TestOfficialXrayCurrentValidation(t *testing.T) {
	binary := requireExternalBinaryVersion(t, "XRAY_CURRENT_BIN", "Xray "+targetXrayCurrentVersion, "version")
	fixtures, err := buildXrayExternalFixtures()
	if err != nil {
		t.Fatal(err.Error())
	}
	runExternalFixtureTable(t, binary, ".json", fixtures, func(binary, configPath, _ string) *exec.Cmd {
		return exec.Command(binary, "run", "-test", "-c", configPath)
	})
}

func TestExternalValidatorRemovesFixtureAndRedactsFailure(t *testing.T) {
	secret := []byte("synthetic-secret-must-not-be-reported")
	var capturedPath string
	workspace, err := validateExternalFixture(os.Args[0], ".json", secret, func(_, configPath, _ string) *exec.Cmd {
		capturedPath = configPath
		return exec.Command(os.Args[0], "-test.run=^$")
	})
	if err != nil {
		t.Fatalf("safe validator helper failed: %s", err.Error())
	}
	if _, statErr := os.Stat(workspace); !os.IsNotExist(statErr) {
		t.Fatal(externalValidationCleanupFailed)
	}
	if _, statErr := os.Stat(capturedPath); !os.IsNotExist(statErr) {
		t.Fatal(externalValidationCleanupFailed)
	}

	_, err = validateExternalFixture(os.Args[0], ".json", secret, func(_, _, _ string) *exec.Cmd {
		return exec.Command(os.Args[0], "-test.run=^$", "-invalid-test-flag")
	})
	if err == nil || err.Error() != externalValidationRejected || strings.Contains(err.Error(), string(secret)) {
		t.Fatal("validator failure was not safely redacted")
	}
}
