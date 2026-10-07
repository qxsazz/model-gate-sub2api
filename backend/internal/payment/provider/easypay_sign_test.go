package provider

import (
	"context"
	"net/url"
	"testing"
)

func TestEasyPaySignConsistentOutput(t *testing.T) {
	t.Parallel()

	params := map[string]string{
		"pid":          "1001",
		"type":         "alipay",
		"out_trade_no": "ORDER123",
		"name":         "Test Product",
		"money":        "10.00",
	}
	pkey := "test_secret_key"

	sign1 := easyPaySign(params, pkey)
	sign2 := easyPaySign(params, pkey)
	if sign1 != sign2 {
		t.Fatalf("easyPaySign should be deterministic: %q != %q", sign1, sign2)
	}
	if len(sign1) != 32 {
		t.Fatalf("MD5 hex should be 32 chars, got %d", len(sign1))
	}
}

func TestEasyPaySignExcludesSignAndSignType(t *testing.T) {
	t.Parallel()

	pkey := "my_key"
	base := map[string]string{
		"pid":  "1001",
		"type": "alipay",
	}
	withSign := map[string]string{
		"pid":       "1001",
		"type":      "alipay",
		"sign":      "should_be_ignored",
		"sign_type": "MD5",
	}

	signBase := easyPaySign(base, pkey)
	signWithExtra := easyPaySign(withSign, pkey)

	if signBase != signWithExtra {
		t.Fatalf("sign and sign_type should be excluded: base=%q, withExtra=%q", signBase, signWithExtra)
	}
}

func TestEasyPaySignExcludesEmptyValues(t *testing.T) {
	t.Parallel()

	pkey := "key123"
	base := map[string]string{
		"pid":  "1001",
		"type": "alipay",
	}
	withEmpty := map[string]string{
		"pid":      "1001",
		"type":     "alipay",
		"device":   "",
		"clientip": "",
	}

	signBase := easyPaySign(base, pkey)
	signWithEmpty := easyPaySign(withEmpty, pkey)

	if signBase != signWithEmpty {
		t.Fatalf("empty values should be excluded: base=%q, withEmpty=%q", signBase, signWithEmpty)
	}
}

func TestEasyPayVerifySignValid(t *testing.T) {
	t.Parallel()

	params := map[string]string{
		"pid":          "1001",
		"type":         "alipay",
		"out_trade_no": "ORDER456",
		"money":        "25.00",
	}
	pkey := "secret"

	sign := easyPaySign(params, pkey)

	// Add sign to params (as would come in a real callback)
	params["sign"] = sign
	params["sign_type"] = "MD5"

	if !easyPayVerifySign(params, pkey, sign) {
		t.Fatal("easyPayVerifySign should return true for a valid signature")
	}
}

func TestEasyPayVerifySignTampered(t *testing.T) {
	t.Parallel()

	params := map[string]string{
		"pid":          "1001",
		"type":         "alipay",
		"out_trade_no": "ORDER789",
		"money":        "50.00",
	}
	pkey := "secret"

	sign := easyPaySign(params, pkey)

	// Tamper with the amount
	params["money"] = "99.99"

	if easyPayVerifySign(params, pkey, sign) {
		t.Fatal("easyPayVerifySign should return false for tampered params")
	}
}

func TestEasyPayVerifySignWrongKey(t *testing.T) {
	t.Parallel()

	params := map[string]string{
		"pid":  "1001",
		"type": "wxpay",
	}

	sign := easyPaySign(params, "correct_key")

	if easyPayVerifySign(params, "wrong_key", sign) {
		t.Fatal("easyPayVerifySign should return false with wrong key")
	}
}

func TestEasyPaySignEmptyParams(t *testing.T) {
	t.Parallel()

	sign := easyPaySign(map[string]string{}, "key123")
	if sign == "" {
		t.Fatal("easyPaySign with empty params should still produce a hash")
	}
	if len(sign) != 32 {
		t.Fatalf("MD5 hex should be 32 chars, got %d", len(sign))
	}
}

func TestEasyPaySignSortOrder(t *testing.T) {
	t.Parallel()

	pkey := "test_key"
	params1 := map[string]string{
		"a": "1",
		"b": "2",
		"c": "3",
	}
	params2 := map[string]string{
		"c": "3",
		"a": "1",
		"b": "2",
	}

	sign1 := easyPaySign(params1, pkey)
	sign2 := easyPaySign(params2, pkey)

	if sign1 != sign2 {
		t.Fatalf("easyPaySign should be order-independent: %q != %q", sign1, sign2)
	}
}

func TestEasyPayVerifySignWrongSignValue(t *testing.T) {
	t.Parallel()

	params := map[string]string{
		"pid":  "1001",
		"type": "alipay",
	}
	pkey := "key"

	if easyPayVerifySign(params, pkey, "00000000000000000000000000000000") {
		t.Fatal("easyPayVerifySign should return false for an incorrect sign value")
	}
}

func TestEasyPayMerchantIdentityMetadata(t *testing.T) {
	t.Parallel()

	provider := &EasyPay{
		config: map[string]string{
			"pid": "1001",
		},
	}

	metadata := provider.MerchantIdentityMetadata()
	if metadata["pid"] != "1001" {
		t.Fatalf("pid = %q, want %q", metadata["pid"], "1001")
	}
}

func TestEasyPayVerifyNotificationAcceptsStandardCallback(t *testing.T) {
	t.Parallel()

	provider := &EasyPay{config: map[string]string{"pid": "1000", "pkey": "merchant-secret"}}
	params := map[string]string{
		"pid":          "1000",
		"trade_no":     "UPSTREAM123",
		"out_trade_no": "ORDER123",
		"type":         "alipay",
		"name":         "balance recharge",
		"money":        "25.00",
		"trade_status": tradeStatusSuccess,
		"param":        "optional-pass-through",
	}
	values := url.Values{}
	for k, v := range params {
		values.Set(k, v)
	}
	values.Set("sign", easyPaySign(params, provider.config["pkey"]))
	values.Set("sign_type", signTypeMD5)

	notification, err := provider.VerifyNotification(context.Background(), values.Encode(), nil)
	if err != nil {
		t.Fatalf("VerifyNotification returned error: %v", err)
	}
	if notification.Status != "success" || notification.OrderID != "ORDER123" || notification.Amount != 25 {
		t.Fatalf("unexpected notification: %+v", notification)
	}
}

func TestEasyPayVerifyNotificationRejectsUnknownParamEvenWhenEmpty(t *testing.T) {
	t.Parallel()

	provider := &EasyPay{config: map[string]string{"pid": "1000", "pkey": "merchant-secret"}}
	params := map[string]string{
		"pid":          "1000",
		"out_trade_no": "ORDER123",
		"money":        "25.00",
		"trade_status": tradeStatusSuccess,
	}
	values := url.Values{}
	for k, v := range params {
		values.Set(k, v)
	}
	values.Set("unexpected", "")
	values.Set("sign", easyPaySign(params, provider.config["pkey"]))
	values.Set("sign_type", signTypeMD5)

	if _, err := provider.VerifyNotification(context.Background(), values.Encode(), nil); err == nil {
		t.Fatal("VerifyNotification should reject callback parameters outside the allowlist")
	}
}

func TestEasyPayVerifyNotificationRejectsReusedPopupSignature(t *testing.T) {
	t.Parallel()

	provider := &EasyPay{config: map[string]string{
		"pid":       "1000",
		"pkey":      "merchant-secret",
		"notifyUrl": "https://site.example.com/api/v1/payment/webhook/easypay",
	}}
	returnURLPrefix := "https://site.example.com/payment/result?order_id=99&out_trade_no=ORDER123&status=success"
	createParams := map[string]string{
		"pid":          "1000",
		"type":         "alipay",
		"out_trade_no": "ORDER123",
		"notify_url":   provider.config["notifyUrl"],
		"return_url":   returnURLPrefix + "&trade_status=" + tradeStatusSuccess,
		"name":         "balance recharge",
		"money":        "650.00",
	}
	exposedPopupSign := easyPaySign(createParams, provider.config["pkey"])

	forgedParams := map[string]string{
		"pid":          createParams["pid"],
		"type":         createParams["type"],
		"out_trade_no": createParams["out_trade_no"],
		"notify_url":   createParams["notify_url"],
		"return_url":   returnURLPrefix,
		"name":         createParams["name"],
		"money":        createParams["money"],
		"trade_status": tradeStatusSuccess,
	}
	if forgedSign := easyPaySign(forgedParams, provider.config["pkey"]); forgedSign != exposedPopupSign {
		t.Fatalf("test setup did not reproduce signature collision: forged=%q popup=%q", forgedSign, exposedPopupSign)
	}

	forged := url.Values{}
	for k, v := range forgedParams {
		forged.Set(k, v)
	}
	forged.Set("sign", exposedPopupSign)
	forged.Set("sign_type", signTypeMD5)

	if _, err := provider.VerifyNotification(context.Background(), forged.Encode(), nil); err == nil {
		t.Fatal("VerifyNotification accepted a payment-page signature replayed as a success callback")
	}
}
