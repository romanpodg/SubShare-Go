package keymanagement

import (
	"fmt"
	"github.com/romanpodg/SubShare-Go/internal/model"
	"net/url"
)

func shadowsocksConnection(patch *model.ShadowsocksStructuredPatch) (url.URL, url.Values, error) {
	var built url.URL
	query := url.Values{}
	if patch == nil {
		return url.URL{}, nil, fmt.Errorf("missing shadowsocks patch payload")
	}
	built.Scheme = "ss"
	built.User = url.UserPassword(setString(patch.Method, "2022-blake3-aes-128-gcm"), setString(patch.Password, ""))
	setIf(query, "plugin", shadowsocksPlugin(patch))
	return built, query, nil
}

func shadowsocksPlugin(patch *model.ShadowsocksStructuredPatch) string {
	plugin := setString(patch.PluginName, "")
	if plugin == "" {
		return ""
	}
	opts := setString(patch.PluginOptions, "")
	if opts != "" {
		plugin += ";" + opts
	}
	return plugin
}
