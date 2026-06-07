package x402

type Challenge struct {
	X402Version int             `json:"x402Version"`
	Resource    Resource        `json:"resource"`
	Accepts     []PaymentOption `json:"accepts"`
}

type Resource struct {
	URL         string `json:"url"`
	Description string `json:"description"`
	MimeType    string `json:"mimeType"`
}

type PaymentOption struct {
	Scheme            string         `json:"scheme"`
	Network           string         `json:"network"`
	Amount            string         `json:"amount"`
	Asset             string         `json:"asset"`
	PayTo             string         `json:"payTo"`
	MaxTimeoutSeconds int            `json:"maxTimeoutSeconds"`
	Extra             map[string]any `json:"extra,omitempty"`
}

type PaymentSignature struct {
	X402Version int              `json:"x402Version"`
	Resource    Resource         `json:"resource,omitempty"`
	Accepted    PaymentOption    `json:"accepted"`
	Extensions  map[string]any   `json:"extensions,omitempty"`
	Payload     SignaturePayload `json:"payload"`
}

type SignaturePayload struct {
	PaymentIndex int      `json:"paymentIndex,omitempty"`
	PaymentGroup []string `json:"paymentGroup,omitempty"`
	Transaction  string   `json:"transaction,omitempty"`
}
