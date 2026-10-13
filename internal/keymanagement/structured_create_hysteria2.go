package keymanagement

import (
	"fmt"
	"github.com/romanpodg/SubShare-Go/internal/model"
	"net/url"
)

func hysteria2Connection(patch *model.Hysteria2StructuredPatch) (url.URL, url.Values, error) {
	var built url.URL
	query := url.Values{}
	if patch == nil {
		return url.URL{}, nil, fmt.Errorf("missing hysteria2 patch payload")
	}
	built.Scheme = "hysteria2"
	built.User = url.User(setString(patch.Authentication, ""))
	setIf(query, "sni", setString(patch.SNI, ""))
	if setBool(patch.Insecure) {
		query.Set("insecure", "1")
	}
	setIf(query, "pinSHA256", setString(patch.CertificateSHA256, ""))
	setIf(query, "obfs", setString(patch.ObfuscationType, ""))
	setIf(query, "obfs-password", setString(patch.ObfuscationPassword, ""))
	return built, query, nil
}
