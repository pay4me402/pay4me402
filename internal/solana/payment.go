package solana

import (
	"context"
	"crypto/ed25519"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha512"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	solanago "github.com/gagliardetto/solana-go"
	computebudget "github.com/gagliardetto/solana-go/programs/compute-budget"
	"github.com/gagliardetto/solana-go/programs/system"
	"github.com/gagliardetto/solana-go/programs/token"
	"github.com/gagliardetto/solana-go/rpc"
	"github.com/majed/payformeproxy/internal/x402"
	"github.com/tyler-smith/go-bip39"
)

func BuildPaymentSignature(ctx context.Context, challenge x402.Challenge, accepted x402.PaymentOption, privateKeyValue string, rpcEndpoint string) (string, string, error) {
	privateKey, err := parsePrivateKey(privateKeyValue)
	if err != nil {
		return "", "", err
	}

	amount, err := strconv.ParseUint(accepted.Amount, 10, 64)
	if err != nil {
		return "", "", fmt.Errorf("parse payment amount: %w", err)
	}

	payTo, err := solanago.PublicKeyFromBase58(accepted.PayTo)
	if err != nil {
		return "", "", fmt.Errorf("parse Solana payTo address: %w", err)
	}

	client := rpc.New(rpcEndpoint)
	recent, err := client.GetLatestBlockhash(ctx, rpc.CommitmentFinalized)
	if err != nil {
		return "", "", fmt.Errorf("fetch recent Solana blockhash: %w", err)
	}

	from := privateKey.PublicKey()
	feePayer := from
	if feePayerValue, _ := accepted.Extra["feePayer"].(string); feePayerValue != "" {
		feePayer, err = solanago.PublicKeyFromBase58(feePayerValue)
		if err != nil {
			return "", "", fmt.Errorf("parse Solana feePayer address: %w", err)
		}
	}
	instructions, err := paymentInstructions(ctx, client, accepted, amount, from, payTo)
	if err != nil {
		return "", "", err
	}

	tx, err := solanago.NewTransaction(
		instructions,
		recent.Value.Blockhash,
		solanago.TransactionPayer(feePayer),
	)
	if err != nil {
		return "", "", fmt.Errorf("build Solana transfer transaction: %w", err)
	}
	tx.Message.SetVersion(solanago.MessageVersionV0)

	_, err = tx.PartialSign(func(key solanago.PublicKey) *solanago.PrivateKey {
		if from.Equals(key) {
			return &privateKey
		}
		return nil
	})
	if err != nil {
		return "", "", fmt.Errorf("sign Solana transfer transaction: %w", err)
	}

	rawTxn, err := tx.MarshalBinary()
	if err != nil {
		return "", "", fmt.Errorf("encode Solana transaction: %w", err)
	}

	signature := x402.PaymentSignature{
		X402Version: challenge.X402Version,
		Resource:    challenge.Resource,
		Accepted:    accepted,
		Payload: x402.SignaturePayload{
			Transaction: base64.StdEncoding.EncodeToString(rawTxn),
		},
	}

	raw, err := json.Marshal(signature)
	if err != nil {
		return "", "", fmt.Errorf("encode PAYMENT-SIGNATURE payload: %w", err)
	}

	return base64.StdEncoding.EncodeToString(raw), "", nil
}

func paymentInstructions(ctx context.Context, client *rpc.Client, accepted x402.PaymentOption, amount uint64, from solanago.PublicKey, payTo solanago.PublicKey) ([]solanago.Instruction, error) {
	if accepted.Asset == "" || accepted.Asset == "SOL" || accepted.Asset == solanago.SolMint.String() {
		return []solanago.Instruction{
			system.NewTransferInstruction(amount, from, payTo).Build(),
		}, nil
	}

	mint, err := solanago.PublicKeyFromBase58(accepted.Asset)
	if err != nil {
		return nil, fmt.Errorf("parse Solana asset mint: %w", err)
	}
	supply, err := client.GetTokenSupply(ctx, mint, rpc.CommitmentFinalized)
	if err != nil {
		return nil, fmt.Errorf("fetch Solana token decimals: %w", err)
	}
	source, _, err := solanago.FindAssociatedTokenAddress(from, mint)
	if err != nil {
		return nil, fmt.Errorf("derive source token account: %w", err)
	}
	destination, _, err := solanago.FindAssociatedTokenAddress(payTo, mint)
	if err != nil {
		return nil, fmt.Errorf("derive destination token account: %w", err)
	}

	return []solanago.Instruction{
		computebudget.NewSetComputeUnitLimitInstruction(20_000).Build(),
		computebudget.NewSetComputeUnitPriceInstruction(1).Build(),
		token.NewTransferCheckedInstruction(amount, supply.Value.Decimals, source, mint, destination, from, nil).Build(),
		memoInstruction(accepted),
	}, nil
}

func memoInstruction(accepted x402.PaymentOption) solanago.Instruction {
	memo := ""
	if memoValue, _ := accepted.Extra["memo"].(string); memoValue != "" {
		memo = memoValue
	} else {
		memoBytes := make([]byte, 16)
		if _, err := rand.Read(memoBytes); err == nil {
			memo = hex.EncodeToString(memoBytes)
		}
	}
	return solanago.NewInstruction(
		solanago.MustPublicKeyFromBase58("MemoSq4gqABAXKb96qnH8TysNcWxMyWCqXgDLGmfcHr"),
		solanago.AccountMetaSlice{},
		[]byte(memo),
	)
}

func parsePrivateKey(value string) (solanago.PrivateKey, error) {
	value = strings.TrimSpace(value)
	if privateKey, err := solanago.PrivateKeyFromBase58(value); err == nil {
		return privateKey, nil
	}
	if !bip39.IsMnemonicValid(value) {
		return nil, fmt.Errorf("load Solana private key: value is neither base58 private key nor valid BIP-39 mnemonic")
	}

	seed := bip39.NewSeed(value, "")
	key, err := deriveEd25519Path(seed, []uint32{44, 501, 0, 0})
	if err != nil {
		return nil, fmt.Errorf("derive Solana private key from mnemonic: %w", err)
	}
	edKey := ed25519.NewKeyFromSeed(key)
	return solanago.PrivateKey(edKey), nil
}

func deriveEd25519Path(seed []byte, path []uint32) ([]byte, error) {
	key, chainCode := hmacSHA512([]byte("ed25519 seed"), seed)
	for _, index := range path {
		data := make([]byte, 1+32+4)
		copy(data[1:], key)
		binary.BigEndian.PutUint32(data[33:], index+0x80000000)
		key, chainCode = hmacSHA512(chainCode, data)
	}
	if len(key) != ed25519.SeedSize {
		return nil, fmt.Errorf("invalid derived key length %d", len(key))
	}
	return key, nil
}

func hmacSHA512(key []byte, data []byte) ([]byte, []byte) {
	mac := hmac.New(sha512.New, key)
	mac.Write(data)
	sum := mac.Sum(nil)
	return sum[:32], sum[32:]
}
