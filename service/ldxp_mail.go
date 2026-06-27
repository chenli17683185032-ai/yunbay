package service

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"html"
	"math"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const ldxpMailExcerptMaxRunes = 500

var (
	ldxpHTMLBreakRe   = regexp.MustCompile(`(?i)<\s*/?\s*(br|p|div|tr|li|table|tbody|thead|section|article|h[1-6])\b[^>]*>`)
	ldxpHTMLTagRe     = regexp.MustCompile(`(?s)<[^>]+>`)
	ldxpWhitespaceRe  = regexp.MustCompile(`[\t\r\f\v ]+`)
	ldxpBlankLineRe   = regexp.MustCompile(`\n{3,}`)
	ldxpOrderRe       = regexp.MustCompile(`(?m)(?:订单号|订单编号|订单)\s*[:：]?\s*(LD[A-Z0-9]+)\b`)
	ldxpCardRe        = regexp.MustCompile(`(?m)(?:卡密账号|卡密|兑换码)\s*[:：]?\s*([^\s]+)`)
	ldxpAmountRe      = regexp.MustCompile(`(?m)(?:支付金额|金额)\s*[:：]?\s*¥?\s*([0-9]+(?:\.[0-9]+)?)\s*(?:元)?`)
	ldxpProductNameRe = regexp.MustCompile(`(?m)(?:商品名称|商品名|购买内容)\s*[:：]?\s*([^\n]+)`)
	ldxpPaidTimeRe    = regexp.MustCompile(`(?m)(?:付款时间|支付时间|付款日期|支付日期)\s*[:：]?\s*(\d{4}-\d{2}-\d{2}\s+\d{2}:\d{2}:\d{2})`)
)

type LdxpParsedMail struct {
	MessageID    string
	ImapUID      string
	RawHash      string
	From         string
	To           string
	Subject      string
	ReceivedTime int64
	OrderNo      string
	Amount       float64
	ProductName  string
	CardKey      string
	PaidTime     int64
	BodyExcerpt  string
}

func ParseLdxpMailText(input string) (*LdxpParsedMail, error) {
	normalized := NormalizeLdxpMailBody(input)
	parsed := &LdxpParsedMail{}

	if matches := ldxpOrderRe.FindStringSubmatch(normalized); len(matches) > 1 {
		parsed.OrderNo = strings.TrimSpace(matches[1])
	}
	if matches := ldxpCardRe.FindStringSubmatch(normalized); len(matches) > 1 {
		parsed.CardKey = strings.TrimSpace(matches[1])
	}
	if matches := ldxpAmountRe.FindStringSubmatch(normalized); len(matches) > 1 {
		amount, err := strconv.ParseFloat(matches[1], 64)
		if err != nil {
			return nil, fmt.Errorf("parse ldxp mail amount: %w", err)
		}
		parsed.Amount = amount
	}
	if matches := ldxpProductNameRe.FindStringSubmatch(normalized); len(matches) > 1 {
		parsed.ProductName = strings.TrimSpace(matches[1])
	}
	if matches := ldxpPaidTimeRe.FindStringSubmatch(normalized); len(matches) > 1 {
		paidTime, err := time.ParseInLocation("2006-01-02 15:04:05", strings.TrimSpace(matches[1]), time.Local)
		if err != nil {
			return nil, fmt.Errorf("parse ldxp mail paid time: %w", err)
		}
		parsed.PaidTime = paidTime.Unix()
	}
	parsed.BodyExcerpt = buildLdxpMailBodyExcerpt(normalized, parsed.CardKey)
	return parsed, nil
}

func NormalizeLdxpMailBody(input string) string {
	body := strings.ReplaceAll(input, "\x00", "")
	body = ldxpHTMLBreakRe.ReplaceAllString(body, "\n")
	body = ldxpHTMLTagRe.ReplaceAllString(body, " ")
	body = html.UnescapeString(body)
	body = strings.ReplaceAll(body, "\u00a0", " ")
	lines := strings.Split(body, "\n")
	for i, line := range lines {
		lines[i] = strings.TrimSpace(ldxpWhitespaceRe.ReplaceAllString(line, " "))
	}
	body = strings.Join(lines, "\n")
	body = ldxpBlankLineRe.ReplaceAllString(body, "\n\n")
	return strings.TrimSpace(body)
}

func HashLdxpMailRaw(raw []byte) string {
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

func SaveLdxpMailEvent(parsed *LdxpParsedMail) (*model.LdxpMailEvent, error) {
	if parsed == nil || strings.TrimSpace(parsed.RawHash) == "" {
		return nil, gorm.ErrInvalidData
	}

	rawHash := strings.TrimSpace(parsed.RawHash)
	if existing, err := getLdxpMailEventByRawHash(rawHash); err == nil {
		return existing, nil
	} else if !isRecordNotFound(err) {
		return nil, err
	}

	messageID := strings.TrimSpace(parsed.MessageID)
	if messageID != "" {
		if existing, err := getLdxpMailEventByMessageID(messageID); err == nil {
			return existing, nil
		} else if !isRecordNotFound(err) {
			return nil, err
		}
	}

	now := common.GetTimestamp()
	event := &model.LdxpMailEvent{
		ImapUid:      strings.TrimSpace(parsed.ImapUID),
		RawHash:      rawHash,
		MailFrom:     strings.TrimSpace(parsed.From),
		MailTo:       strings.TrimSpace(parsed.To),
		Subject:      strings.TrimSpace(parsed.Subject),
		ReceivedTime: parsed.ReceivedTime,
		OrderNo:      strings.TrimSpace(parsed.OrderNo),
		Amount:       parsed.Amount,
		ProductName:  strings.TrimSpace(parsed.ProductName),
		CardKey:      strings.TrimSpace(parsed.CardKey),
		PaidTime:     parsed.PaidTime,
		BodyExcerpt:  strings.TrimSpace(parsed.BodyExcerpt),
		CreatedTime:  now,
	}
	if messageID != "" {
		event.MessageId = &messageID
	}
	if err := model.InsertLdxpMailEvent(event); err != nil {
		if existing, findErr := getLdxpMailEventByRawHash(rawHash); findErr == nil {
			return existing, nil
		}
		if messageID != "" {
			if existing, findErr := getLdxpMailEventByMessageID(messageID); findErr == nil {
				return existing, nil
			}
		}
		return nil, err
	}
	return event, nil
}

func TryMatchLdxpMailEvent(event *model.LdxpMailEvent) (*model.LdxpTopupSession, error) {
	if event == nil {
		return nil, gorm.ErrInvalidData
	}
	orderNo := strings.TrimSpace(event.OrderNo)
	if orderNo == "" {
		return nil, fmt.Errorf("%w: missing mail order no", ErrLdxpInvalidSessionRequest)
	}

	var matched *model.LdxpTopupSession
	err := model.DB.Transaction(func(tx *gorm.DB) error {
		var session model.LdxpTopupSession
		query := tx.Where("worker_order_no = ? AND status IN ?", orderNo, []string{model.LdxpStatusWorkerPaid, model.LdxpStatusVerifyFailed}).Order("updated_time DESC, id DESC")
		if !common.UsingSQLite {
			query = query.Clauses(clause.Locking{Strength: "UPDATE"})
		}
		if err := query.First(&session).Error; err != nil {
			return err
		}

		if strings.TrimSpace(session.WorkerCardKey) != "" && strings.TrimSpace(event.CardKey) != "" && strings.TrimSpace(session.WorkerCardKey) != strings.TrimSpace(event.CardKey) {
			return fmt.Errorf("%w: ldxp mail card mismatch", ErrLdxpInvalidSessionRequest)
		}
		if session.WorkerAmount > 0 && event.Amount > 0 && math.Abs(session.WorkerAmount-event.Amount) > 0.01 {
			return fmt.Errorf("%w: ldxp mail amount mismatch", ErrLdxpInvalidSessionRequest)
		}

		messageID := ""
		if event.MessageId != nil {
			messageID = strings.TrimSpace(*event.MessageId)
		}
		now := common.GetTimestamp()
		updates := map[string]interface{}{
			"mail_message_id":    messageID,
			"mail_order_no":      orderNo,
			"mail_amount":        event.Amount,
			"mail_product_name":  strings.TrimSpace(event.ProductName),
			"mail_card_key":      strings.TrimSpace(event.CardKey),
			"mail_from":          strings.TrimSpace(event.MailFrom),
			"mail_to":            strings.TrimSpace(event.MailTo),
			"mail_subject":       strings.TrimSpace(event.Subject),
			"mail_received_time": event.ReceivedTime,
			"updated_time":       now,
		}
		if err := tx.Model(&model.LdxpTopupSession{}).Where("id = ?", session.Id).Updates(updates).Error; err != nil {
			return err
		}
		if err := tx.Model(&model.LdxpMailEvent{}).Where("id = ?", event.Id).Updates(map[string]interface{}{
			"matched_session_id": session.SessionId,
			"processed":          true,
			"error_message":      "",
		}).Error; err != nil {
			return err
		}
		if err := tx.Where("id = ?", session.Id).First(&session).Error; err != nil {
			return err
		}
		matched = &session
		return nil
	})
	if err != nil {
		return nil, err
	}
	return matched, nil
}

func buildLdxpMailBodyExcerpt(body string, cardKey string) string {
	excerpt := body
	cardKey = strings.TrimSpace(cardKey)
	if cardKey != "" {
		excerpt = strings.ReplaceAll(excerpt, cardKey, RedactLdxpValue(cardKey))
	}
	runes := []rune(excerpt)
	if len(runes) > ldxpMailExcerptMaxRunes {
		excerpt = string(runes[:ldxpMailExcerptMaxRunes])
	}
	if cardKey != "" && strings.Contains(excerpt, cardKey) {
		excerpt = strings.ReplaceAll(excerpt, cardKey, "[redacted]")
	}
	return strings.TrimSpace(excerpt)
}

func getLdxpMailEventByRawHash(rawHash string) (*model.LdxpMailEvent, error) {
	var event model.LdxpMailEvent
	err := model.DB.Where("raw_hash = ?", rawHash).First(&event).Error
	return &event, err
}

func getLdxpMailEventByMessageID(messageID string) (*model.LdxpMailEvent, error) {
	var event model.LdxpMailEvent
	err := model.DB.Where("message_id = ?", messageID).First(&event).Error
	return &event, err
}

func isRecordNotFound(err error) bool {
	return errors.Is(err, gorm.ErrRecordNotFound)
}
