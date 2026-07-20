package main

import (
	"crypto/tls"
	"encoding/base64"
	"errors"
	"fmt"
	"net"
	"net/smtp"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
)

type campaignEmailSender func(subject string, receiver string, content string) error

type campaignEmailFatalError struct {
	err error
}

func (err *campaignEmailFatalError) Error() string {
	return err.err.Error()
}

func (err *campaignEmailFatalError) Unwrap() error {
	return err.err
}

type qqCampaignSMTPSession struct {
	client *smtp.Client
	conn   net.Conn
}

func newQQCampaignSMTPSession() (*qqCampaignSMTPSession, error) {
	if !strings.EqualFold(common.SMTPServer, "smtp.qq.com") || common.SMTPPort != 587 || common.SMTPSSLEnabled {
		return nil, errors.New("QQ campaign SMTP requires smtp.qq.com:587 with STARTTLS")
	}
	address := net.JoinHostPort(common.SMTPServer, fmt.Sprintf("%d", common.SMTPPort))
	conn, err := (&net.Dialer{Timeout: 20 * time.Second}).Dial("tcp", address)
	if err != nil {
		return nil, err
	}
	if err = conn.SetDeadline(time.Now().Add(30 * time.Second)); err != nil {
		_ = conn.Close()
		return nil, err
	}
	client, err := smtp.NewClient(conn, common.SMTPServer)
	if err != nil {
		_ = conn.Close()
		return nil, err
	}
	closeOnError := func(err error) (*qqCampaignSMTPSession, error) {
		_ = client.Close()
		return nil, err
	}
	if err = client.StartTLS(&tls.Config{ServerName: common.SMTPServer, MinVersion: tls.VersionTLS12}); err != nil {
		return closeOnError(err)
	}
	if err = client.Auth(smtp.PlainAuth("", common.SMTPAccount, common.SMTPToken, common.SMTPServer)); err != nil {
		return closeOnError(err)
	}
	return &qqCampaignSMTPSession{client: client, conn: conn}, nil
}

func (session *qqCampaignSMTPSession) Send(subject string, receiver string, content string) error {
	message, err := buildCampaignSMTPMessage(subject, receiver, content)
	if err != nil {
		return err
	}
	if err = session.conn.SetDeadline(time.Now().Add(30 * time.Second)); err != nil {
		return err
	}
	if err = session.client.Mail(common.SMTPFrom); err == nil {
		err = session.client.Rcpt(receiver)
	}
	if err == nil {
		var writer interface {
			Write([]byte) (int, error)
			Close() error
		}
		writer, err = session.client.Data()
		if err == nil {
			_, writeErr := writer.Write(message)
			closeErr := writer.Close()
			if writeErr != nil {
				err = writeErr
			} else {
				err = closeErr
			}
		}
	}
	if err == nil {
		return nil
	}
	if resetErr := session.client.Reset(); resetErr != nil {
		return &campaignEmailFatalError{err: fmt.Errorf("QQ SMTP session became unusable after %v: %w", err, resetErr)}
	}
	return err
}

func (session *qqCampaignSMTPSession) Close() error {
	if session == nil || session.client == nil {
		return nil
	}
	_ = session.conn.SetDeadline(time.Now().Add(10 * time.Second))
	if err := session.client.Quit(); err != nil {
		_ = session.conn.Close()
		return err
	}
	return nil
}

func buildCampaignSMTPMessage(subject string, receiver string, content string) ([]byte, error) {
	separator := strings.LastIndex(common.SMTPFrom, "@")
	if separator <= 0 || separator == len(common.SMTPFrom)-1 {
		return nil, errors.New("SMTP sender address is invalid")
	}
	encodedSubject := base64.StdEncoding.EncodeToString([]byte(subject))
	messageID := fmt.Sprintf("<%d.%s@%s>", time.Now().UnixNano(), common.GetRandomString(12), common.SMTPFrom[separator+1:])
	return []byte(fmt.Sprintf("To: %s\r\n"+
		"From: %s <%s>\r\n"+
		"Subject: =?UTF-8?B?%s?=\r\n"+
		"Date: %s\r\n"+
		"Message-ID: %s\r\n"+
		"Content-Type: text/html; charset=UTF-8\r\n\r\n%s\r\n",
		receiver, common.SystemName, common.SMTPFrom, encodedSubject,
		time.Now().Format(time.RFC1123Z), messageID, content)), nil
}
