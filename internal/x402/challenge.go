package x402

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

func DecodePaymentRequired(header string) (Challenge, error) {
	if header == "" {
		return Challenge{}, errors.New("402 response did not include Payment-Required header")
	}

	raw, err := base64.StdEncoding.DecodeString(header)
	if err != nil {
		return Challenge{}, fmt.Errorf("decode Payment-Required header: %w", err)
	}

	var challenge Challenge
	if err := json.Unmarshal(raw, &challenge); err != nil {
		return Challenge{}, fmt.Errorf("parse Payment-Required JSON: %w", err)
	}

	return challenge, nil
}

func SelectAlgorandPayment(challenge Challenge) (PaymentOption, error) {
	for _, option := range challenge.Accepts {
		if IsAlgorandPayment(option) {
			return option, nil
		}
	}

	return PaymentOption{}, errors.New("Payment-Required header did not include an Algorand exact payment option")
}

func SelectSolanaPayment(challenge Challenge) (PaymentOption, error) {
	for _, option := range challenge.Accepts {
		if IsSolanaPayment(option) {
			return option, nil
		}
	}

	return PaymentOption{}, errors.New("Payment-Required header did not include a Solana exact payment option")
}

func IsAlgorandPayment(option PaymentOption) bool {
	return option.Scheme == "exact" && strings.HasPrefix(option.Network, "algorand:")
}

func IsSolanaPayment(option PaymentOption) bool {
	return option.Scheme == "exact" && strings.HasPrefix(option.Network, "solana")
}
