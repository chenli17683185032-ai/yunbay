package service

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/shopspring/decimal"
)

type ParsedLdxpMail struct {
	ProductName   string
	PaidCents     int64
	Quantity      int
	PaymentTime   int64
	OrderNo       string
	ContentMasked string
}

var (
	mailBRTagRegexp       = regexp.MustCompile(`(?i)<br\s*/?>`)
	mailHTMLTagRegexp     = regexp.MustCompile(`(?s)<[^>]*>`)
	mailMoneyRegexp       = regexp.MustCompile(`[0-9]+(?:\.[0-9]+)?`)
	ldxpPaidRegexp        = regexp.MustCompile(`实付\s*[:：]?\s*([0-9]+(?:\.[0-9]+)?)\s*元?`)
	ldxpQuantityRegexp    = regexp.MustCompile(`数量\s*[:：]?\s*([0-9]+)\s*[，,]?`)
	ldxpPaymentTimeRegexp = regexp.MustCompile(`付款时间\s*[:：]?\s*([0-9]{4}-[0-9]{2}-[0-9]{2} [0-9]{2}:[0-9]{2}:[0-9]{2})`)
	ldxpOrderNoRegexp     = regexp.MustCompile(`单号\s*[:：]?\s*([^\s，,]+)`)
)

func MoneyTextToCents(input string) (int64, error) {
	moneyText := mailMoneyRegexp.FindString(input)
	if moneyText == "" {
		return 0, fmt.Errorf("money amount missing: %q", input)
	}

	amount, err := decimal.NewFromString(moneyText)
	if err != nil {
		return 0, err
	}

	return amount.Mul(decimal.NewFromInt(100)).Round(0).IntPart(), nil
}

func ParseLdxpOrderMail(raw string) (*ParsedLdxpMail, error) {
	text := normalizeMailText(raw)
	paidMatch := ldxpPaidRegexp.FindStringSubmatch(text)
	if len(paidMatch) < 2 {
		return nil, errors.New("ldxp mail paid amount missing")
	}
	paidCents, err := MoneyTextToCents(paidMatch[1])
	if err != nil {
		return nil, err
	}

	orderMatch := ldxpOrderNoRegexp.FindStringSubmatch(text)
	if len(orderMatch) < 2 || strings.TrimSpace(orderMatch[1]) == "" {
		return nil, errors.New("ldxp mail order number missing")
	}

	mail := &ParsedLdxpMail{
		ProductName: extractLdxpProductName(text),
		PaidCents:   paidCents,
		OrderNo:     strings.Trim(strings.TrimSpace(orderMatch[1]), "，,。.;；"),
	}

	if quantityMatch := ldxpQuantityRegexp.FindStringSubmatch(text); len(quantityMatch) >= 2 {
		_, _ = fmt.Sscanf(quantityMatch[1], "%d", &mail.Quantity)
	}

	if timeMatch := ldxpPaymentTimeRegexp.FindStringSubmatch(text); len(timeMatch) >= 2 {
		loc := time.FixedZone("Asia/Shanghai", 8*60*60)
		paymentTime, err := time.ParseInLocation("2006-01-02 15:04:05", timeMatch[1], loc)
		if err != nil {
			return nil, err
		}
		mail.PaymentTime = paymentTime.Unix()
	}

	mail.ContentMasked = maskPurchaseContent(extractLdxpPurchaseContent(text))
	return mail, nil
}

func normalizeMailText(raw string) string {
	text := strings.ReplaceAll(raw, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	text = mailBRTagRegexp.ReplaceAllString(text, "\n")
	text = mailHTMLTagRegexp.ReplaceAllString(text, "")
	return text
}

func extractLdxpProductName(text string) string {
	const prefix = "感谢购买商品"
	idx := strings.Index(text, prefix)
	if idx < 0 {
		return ""
	}
	rest := text[idx+len(prefix):]
	if lineEnd := strings.IndexByte(rest, '\n'); lineEnd >= 0 {
		rest = rest[:lineEnd]
	}
	return strings.TrimSpace(rest)
}

func extractLdxpPurchaseContent(text string) string {
	const marker = "以下是您的购买内容"
	idx := strings.Index(text, marker)
	if idx < 0 {
		return ""
	}
	content := text[idx+len(marker):]
	content = strings.TrimLeft(content, " \t\n:：，,。.;；")
	return strings.TrimSpace(content)
}

func maskPurchaseContent(content string) string {
	runes := []rune(strings.TrimSpace(content))
	if len(runes) <= 8 {
		return string(runes)
	}
	return string(runes[:4]) + "********" + string(runes[len(runes)-4:])
}
