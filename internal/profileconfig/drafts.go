package profileconfig

// VMessConfigPayload holds decoded legacy vmess:// JSON payload fields.
type VMessConfigPayload struct {
	Address string `json:"add"`
	Port    string `json:"port"`
	ID      string `json:"id"`
	Name    string `json:"ps"`
}

// LinkConfigurationDraft represents an unrolled, normalized configuration draft.
type LinkConfigurationDraft struct {
	Protocol          string
	Server            string
	Port              int
	Identifier        string
	Remark            string
	ServerDescription string
	Network           string
	Security          string
	Path              string
	Host              string
	SNI               string
	ALPN              string
	Flow              string
	Encryption        string
	Fingerprint       string
	PublicKey         string
	ShortID           string
	SpiderX           string
	AllowInsecure     bool
	GRPCServiceName   string
	VMessSecurity     string
	VMessAlterID      string
	HeaderType        string
}
