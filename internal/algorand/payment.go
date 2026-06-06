package algorand

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"

	"github.com/algorand/go-algorand-sdk/v2/client/v2/algod"
	"github.com/algorand/go-algorand-sdk/v2/crypto"
	"github.com/algorand/go-algorand-sdk/v2/encoding/msgpack"
	"github.com/algorand/go-algorand-sdk/v2/mnemonic"
	"github.com/algorand/go-algorand-sdk/v2/transaction"
	"github.com/algorand/go-algorand-sdk/v2/types"
	"github.com/majed/payformeproxy/internal/env"
	"github.com/majed/payformeproxy/internal/x402"
)

func BuildPaymentSignature(challenge x402.Challenge, accepted x402.PaymentOption, mn string) (string, string, error) {
	account, err := mnemonic.ToPrivateKey(mn)
	if err != nil {
		return "", "", fmt.Errorf("load Algorand account from mnemonic: %w", err)
	}

	amount, err := strconv.ParseUint(accepted.Amount, 10, 64)
	if err != nil {
		return "", "", fmt.Errorf("parse payment amount: %w", err)
	}

	assetID, err := strconv.ParseUint(accepted.Asset, 10, 64)
	if err != nil {
		return "", "", fmt.Errorf("parse ASA asset id: %w", err)
	}

	algodClient, err := algod.MakeClient(env.Get("ALGOD_ADDRESS", "https://testnet-api.algonode.cloud"), os.Getenv("ALGOD_TOKEN"))
	if err != nil {
		return "", "", fmt.Errorf("create algod client: %w", err)
	}

	params, err := algodClient.SuggestedParams().Do(context.Background())
	if err != nil {
		return "", "", fmt.Errorf("fetch suggested Algorand params: %w", err)
	}

	sender, err := crypto.GenerateAddressFromSK(account)
	if err != nil {
		return "", "", fmt.Errorf("derive Algorand sender address: %w", err)
	}

	feePayer, _ := accepted.Extra["feePayer"].(string)
	if feePayer == "" {
		return "", "", errors.New("Algorand payment option did not include extra.feePayer")
	}

	feeParams := params
	feeParams.FlatFee = true
	feeParams.Fee = types.MicroAlgos(params.MinFee * 2)

	paymentParams := params
	paymentParams.FlatFee = true
	paymentParams.Fee = 0

	feeTxn, err := transaction.MakePaymentTxn(
		feePayer,
		feePayer,
		0,
		nil,
		"",
		feeParams,
	)
	if err != nil {
		return "", "", fmt.Errorf("build facilitator fee transaction: %w", err)
	}

	txn, err := transaction.MakeAssetTransferTxn(
		sender.String(),
		accepted.PayTo,
		amount,
		nil,
		paymentParams,
		"",
		assetID,
	)
	if err != nil {
		return "", "", fmt.Errorf("build ASA transfer: %w", err)
	}

	groupID, err := crypto.ComputeGroupID([]types.Transaction{feeTxn, txn})
	if err != nil {
		return "", "", fmt.Errorf("compute payment group id: %w", err)
	}
	feeTxn.Group = groupID
	txn.Group = groupID

	txID, signedTxn, err := crypto.SignTransaction(account, txn)
	if err != nil {
		return "", "", fmt.Errorf("sign ASA transfer: %w", err)
	}

	signature := x402.PaymentSignature{
		X402Version:  challenge.X402Version,
		Scheme:       accepted.Scheme,
		Network:      accepted.Network,
		Resource:     challenge.Resource,
		Accepted:     accepted,
		Extensions:   map[string]any{},
		OutputSchema: nil,
		Payload: x402.SignaturePayload{
			PaymentIndex: 1,
			PaymentGroup: []string{
				base64.StdEncoding.EncodeToString(msgpack.Encode(feeTxn)),
				base64.StdEncoding.EncodeToString(signedTxn),
			},
		},
	}

	raw, err := json.Marshal(signature)
	if err != nil {
		return "", "", fmt.Errorf("encode PAYMENT-SIGNATURE payload: %w", err)
	}

	return base64.StdEncoding.EncodeToString(raw), txID, nil
}
