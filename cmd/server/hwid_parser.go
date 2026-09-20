package main

import (
	"net/http"
	"net/url"
	"regexp"
	"strings"
)

type ParsedDeviceInfo struct {
	ClientApp     string
	ClientVersion string
	Platform      string
	OSVersion     string
	DeviceModel   string
	DeviceBrand   string
	NormalizedID  string
}

var (
	reV2rayNGVersion    = regexp.MustCompile(`(?i)\bv2rayng/([0-9][0-9a-z._-]*)`)
	reHappVersion       = regexp.MustCompile(`(?i)\bhapp(?:proxy)?/([0-9][0-9a-z._-]*)`)
	reHiddifyVersion    = regexp.MustCompile(`(?i)\bhiddify(?:next)?/([0-9][0-9a-z._-]*)`)
	reShadowrocketVer   = regexp.MustCompile(`(?i)\bshadowrocket/([0-9][0-9a-z._-]*)`)
	reStreisandVer      = regexp.MustCompile(`(?i)\bstreisand/([0-9][0-9a-z._-]*)`)
	reAndroidVersion    = regexp.MustCompile(`(?i)android\s+([0-9][0-9a-z._-]*)`)
	reAndroidModelBuild = regexp.MustCompile(`(?i);\s*([^;()]+?)\s+build/([a-z0-9-]+)`)
	reIOSVersion        = regexp.MustCompile(`(?i)(?:iphone\s+os|cpu\s+os)\s+([0-9_]+)`)
	reWindowsVersion    = regexp.MustCompile(`(?i)windows\s+nt\s+([0-9.]+)`)
	reMacOSVersion      = regexp.MustCompile(`(?i)mac\s+os\s+x\s+([0-9_]+)`)
)

func normalizeHWID(raw string) string {
	return strings.ToLower(strings.TrimSpace(raw))
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return ""
}

func extractValue(values url.Values, headers http.Header, queryKeys []string, headerKeys []string) string {
	for _, key := range queryKeys {
		if value := strings.TrimSpace(values.Get(key)); value != "" {
			return value
		}
	}
	for _, key := range headerKeys {
		if value := strings.TrimSpace(headers.Get(key)); value != "" {
			return strings.Trim(value, `"`)
		}
	}
	return ""
}

// clientAppSignatures lists user-agent substrings that identify a client app,
// in detection order, with the regexp that extracts its version.
var clientAppSignatures = []struct {
	needles []string
	app     string
	version *regexp.Regexp
}{
	{[]string{"happproxy", "happ/"}, "Happ", reHappVersion},
	{[]string{"hiddify"}, "Hiddify", reHiddifyVersion},
	{[]string{"shadowrocket"}, "Shadowrocket", reShadowrocketVer},
	{[]string{"streisand"}, "Streisand", reStreisandVer},
}

// desktopPlatformSignatures lists OS version regexps tried after Android, in
// detection order. dotted converts "17_1" style versions to "17.1".
var desktopPlatformSignatures = []struct {
	version  *regexp.Regexp
	platform string
	dotted   bool
}{
	{reIOSVersion, "iOS", true},
	{reWindowsVersion, "Windows", false},
	{reMacOSVersion, "macOS", true},
}

func containsAny(haystack string, needles []string) bool {
	for _, needle := range needles {
		if strings.Contains(haystack, needle) {
			return true
		}
	}
	return false
}

func detectClientApp(ua, uaLower string, info *ParsedDeviceInfo) {
	if matches := reV2rayNGVersion.FindStringSubmatch(ua); len(matches) > 1 {
		info.ClientApp = "v2rayNG"
		info.ClientVersion = matches[1]
		return
	}
	if strings.Contains(uaLower, "dalvik/") && strings.Contains(uaLower, "android") {
		info.ClientApp = "v2rayNG"
		return
	}
	for _, sig := range clientAppSignatures {
		if !containsAny(uaLower, sig.needles) {
			continue
		}
		info.ClientApp = sig.app
		if matches := sig.version.FindStringSubmatch(ua); len(matches) > 1 {
			info.ClientVersion = matches[1]
		}
		return
	}
}

func detectPlatform(ua, uaLower string, info *ParsedDeviceInfo) {
	if matches := reAndroidVersion.FindStringSubmatch(ua); len(matches) > 1 {
		info.Platform = "Android"
		info.OSVersion = matches[1]
		detectAndroidDevice(ua, info)
		return
	}
	for _, sig := range desktopPlatformSignatures {
		matches := sig.version.FindStringSubmatch(ua)
		if len(matches) <= 1 {
			continue
		}
		info.Platform = sig.platform
		info.OSVersion = matches[1]
		if sig.dotted {
			info.OSVersion = strings.ReplaceAll(matches[1], "_", ".")
		}
		return
	}
	if strings.Contains(uaLower, "linux") {
		info.Platform = "Linux"
	}
}

// detectAndroidDevice fills the model from the "; <model> Build/<tag>" segment
// and derives the brand when the build tag ends with the model name.
func detectAndroidDevice(ua string, info *ParsedDeviceInfo) {
	modelBuild := reAndroidModelBuild.FindStringSubmatch(ua)
	if len(modelBuild) <= 2 {
		return
	}
	info.DeviceModel = strings.TrimSpace(modelBuild[1])
	buildTag := strings.TrimSpace(modelBuild[2])
	if buildTag == "" {
		return
	}
	upperModel := strings.ToUpper(strings.ReplaceAll(info.DeviceModel, " ", ""))
	upperBuild := strings.ToUpper(strings.ReplaceAll(buildTag, " ", ""))
	if upperModel == "" || !strings.HasSuffix(upperBuild, upperModel) {
		return
	}
	if brand := strings.TrimSpace(strings.TrimSuffix(upperBuild, upperModel)); brand != "" {
		info.DeviceBrand = brand
	}
}

func ParseDeviceInfo(hwid string, ua string, headers http.Header, queryParams url.Values) ParsedDeviceInfo {
	info := ParsedDeviceInfo{NormalizedID: normalizeHWID(hwid)}
	ua = strings.TrimSpace(ua)
	uaLower := strings.ToLower(ua)

	detectClientApp(ua, uaLower, &info)
	detectPlatform(ua, uaLower, &info)

	explicitPlatform := extractValue(queryParams, headers,
		[]string{"platform", "device_os", "os"},
		[]string{"X-Device-Platform", "X-Device-OS", "X-OS", "X-Ver-OS", "Sec-CH-UA-Platform"},
	)
	explicitOSVersion := extractValue(queryParams, headers,
		[]string{"os_version", "ver_os"},
		[]string{"X-OS-Version", "X-Ver-OS-Version", "X-Ver-OS"},
	)
	explicitDeviceModel := extractValue(queryParams, headers,
		[]string{"device_model", "model"},
		[]string{"X-Device-Model"},
	)
	explicitAppName := extractValue(queryParams, headers,
		[]string{"app_name"},
		[]string{"X-App-Name"},
	)
	explicitAppVersion := extractValue(queryParams, headers,
		[]string{"app_version"},
		[]string{"X-App-Version"},
	)

	info.Platform = firstNonEmpty(explicitPlatform, info.Platform)
	info.OSVersion = firstNonEmpty(explicitOSVersion, info.OSVersion)
	info.DeviceModel = firstNonEmpty(explicitDeviceModel, info.DeviceModel)
	info.ClientApp = firstNonEmpty(explicitAppName, info.ClientApp)
	info.ClientVersion = firstNonEmpty(explicitAppVersion, info.ClientVersion)

	return info
}
