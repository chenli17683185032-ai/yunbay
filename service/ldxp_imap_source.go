package service

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/emersion/go-imap/v2"
	"github.com/emersion/go-imap/v2/imapclient"
	"github.com/emersion/go-message/mail"
)

const ldxpIMAPFetchLimit = 200

type LdxpIMAPConfig struct {
	Host     string
	Port     int
	Username string
	Password string
	Mailbox  string
}

func LdxpIMAPConfigFromEnv() LdxpIMAPConfig {
	port, _ := strconv.Atoi(os.Getenv("LDXP_MAIL_IMAP_PORT"))
	if port == 0 {
		port = 993
	}
	mailbox := os.Getenv("LDXP_MAIL_IMAP_MAILBOX")
	if mailbox == "" {
		mailbox = "INBOX"
	}
	return LdxpIMAPConfig{
		Host:     strings.TrimSpace(os.Getenv("LDXP_MAIL_IMAP_HOST")),
		Port:     port,
		Username: strings.TrimSpace(os.Getenv("LDXP_MAIL_IMAP_USER")),
		Password: os.Getenv("LDXP_MAIL_IMAP_PASSWORD"),
		Mailbox:  mailbox,
	}
}

func (c LdxpIMAPConfig) Enabled() bool {
	return c.Host != "" && c.Username != "" && c.Password != ""
}

func ConfiguredLdxpMailSource() LdxpMailSource {
	cfg := LdxpIMAPConfigFromEnv()
	if !cfg.Enabled() {
		return StoredLdxpMailSource{}
	}
	return NewLdxpIMAPSource(cfg)
}

type LdxpIMAPSource struct {
	cfg LdxpIMAPConfig
}

func NewLdxpIMAPSource(cfg LdxpIMAPConfig) *LdxpIMAPSource {
	return &LdxpIMAPSource{cfg: cfg}
}

func (s *LdxpIMAPSource) FetchRecent(ctx context.Context) ([]*model.LdxpMailEvent, error) {
	if s == nil || !s.cfg.Enabled() {
		return StoredLdxpMailSource{}.FetchRecent(ctx)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	client, err := imapclient.DialTLS(net.JoinHostPort(s.cfg.Host, strconv.Itoa(s.cfg.Port)), &imapclient.Options{
		TLSConfig: &tls.Config{ServerName: s.cfg.Host, MinVersion: tls.VersionTLS12},
		Dialer:    imapDialerFromContext(ctx),
	})
	if err != nil {
		return nil, err
	}
	defer client.Close()

	if err := client.Login(s.cfg.Username, s.cfg.Password).Wait(); err != nil {
		return nil, fmt.Errorf("ldxp imap login failed for %s: %w", s.cfg.Username, err)
	}
	defer client.Logout().Wait()

	selected, err := client.Select(s.cfg.Mailbox, &imap.SelectOptions{ReadOnly: true}).Wait()
	if err != nil {
		return nil, fmt.Errorf("ldxp imap select mailbox %q failed: %w", s.cfg.Mailbox, err)
	}
	if selected.NumMessages == 0 {
		return nil, nil
	}

	seqSet := latestIMAPSeqSet(selected.NumMessages, ldxpIMAPFetchLimit)
	bodySection := &imap.FetchItemBodySection{Peek: true}
	messages, err := client.Fetch(seqSet, &imap.FetchOptions{
		UID:         true,
		Envelope:    true,
		BodySection: []*imap.FetchItemBodySection{bodySection},
	}).Collect()
	if err != nil {
		return nil, err
	}

	events := make([]*model.LdxpMailEvent, 0, len(messages))
	for _, message := range messages {
		if err := ctx.Err(); err != nil {
			return events, err
		}
		raw := message.FindBodySection(bodySection)
		if len(raw) == 0 {
			continue
		}
		event, err := ldxpMailEventFromIMAPMessage(s.cfg.Username, message, raw)
		if err != nil {
			common.SysLog("ldxp imap mail parse skipped: " + err.Error())
			continue
		}
		if err := model.DB.WithContext(ctx).Where("raw_hash = ?", event.RawHash).FirstOrCreate(event).Error; err != nil {
			return events, err
		}
		events = append(events, event)
	}

	return events, nil
}

func imapDialerFromContext(ctx context.Context) *net.Dialer {
	timeout := 30 * time.Second
	if deadline, ok := ctx.Deadline(); ok {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			remaining = time.Nanosecond
		}
		if remaining < timeout {
			timeout = remaining
		}
	}
	return &net.Dialer{Timeout: timeout}
}

func latestIMAPSeqSet(total uint32, limit uint32) imap.SeqSet {
	if limit == 0 || limit > total {
		limit = total
	}
	start := total - limit + 1
	return imap.SeqSetNum(start, total)
}

func ldxpMailEventFromIMAPMessage(sourceAccount string, message *imapclient.FetchMessageBuffer, raw []byte) (*model.LdxpMailEvent, error) {
	bodyText := extractMailText(raw)
	parsed, err := ParseLdxpOrderMail(bodyText)
	if err != nil {
		return nil, err
	}

	now := common.GetTimestamp()
	event := &model.LdxpMailEvent{
		RawHash:     HashLdxpMailRaw(raw),
		ProductName: parsed.ProductName,
		OrderNo:     parsed.OrderNo,
		Amount:      float64(parsed.PaidCents) / 100,
		PaidTime:    parsed.PaymentTime,
		BodyExcerpt: parsed.ContentMasked,
		MailTo:      sourceAccount,
		CreatedTime: now,
	}
	if message != nil {
		if message.UID != 0 {
			event.ImapUid = strconv.FormatUint(uint64(message.UID), 10)
		}
		if message.Envelope != nil {
			if message.Envelope.MessageID != "" {
				messageID := message.Envelope.MessageID
				event.MessageId = &messageID
			}
			event.Subject = message.Envelope.Subject
			if len(message.Envelope.From) > 0 {
				event.MailFrom = message.Envelope.From[0].Addr()
			}
			if !message.Envelope.Date.IsZero() {
				event.ReceivedTime = firstNonZero(event.ReceivedTime, message.Envelope.Date.Unix())
			}
		}
	}
	return event, nil
}

func firstNonZero(primary int64, fallback int64) int64 {
	if primary != 0 {
		return primary
	}
	return fallback
}

func extractMailText(raw []byte) string {
	reader, err := mail.CreateReader(bytes.NewReader(raw))
	if err != nil || reader == nil {
		return string(raw)
	}
	defer reader.Close()

	var parts []string
	for {
		part, err := reader.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil || part == nil {
			break
		}
		if _, ok := part.Header.(*mail.InlineHeader); !ok {
			continue
		}
		body, err := io.ReadAll(part.Body)
		if err != nil {
			continue
		}
		if text := strings.TrimSpace(string(body)); text != "" {
			parts = append(parts, text)
		}
	}
	if len(parts) == 0 {
		return string(raw)
	}
	return strings.Join(parts, "\n")
}

var _ LdxpMailSource = (*LdxpIMAPSource)(nil)
var _ LdxpMailSource = StoredLdxpMailSource{}
