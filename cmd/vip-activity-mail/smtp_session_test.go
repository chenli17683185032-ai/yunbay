package main

import (
	"encoding/base64"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/require"
)

func TestBuildCampaignSMTPMessageMatchesCampaignContent(t *testing.T) {
	originalFrom := common.SMTPFrom
	originalSystemName := common.SystemName
	common.SMTPFrom = "sender@example.com"
	common.SystemName = "yunbay"
	t.Cleanup(func() {
		common.SMTPFrom = originalFrom
		common.SystemName = originalSystemName
	})

	message, err := buildCampaignSMTPMessage(campaignSubject, "member@example.com", campaignHTML)
	require.NoError(t, err)
	text := string(message)
	require.Contains(t, text, "To: member@example.com\r\n")
	require.Contains(t, text, "From: yunbay <sender@example.com>\r\n")
	require.Contains(t, text, "Subject: =?UTF-8?B?"+base64.StdEncoding.EncodeToString([]byte(campaignSubject))+"?=\r\n")
	require.Contains(t, text, "Content-Type: text/html; charset=UTF-8\r\n")
	require.Contains(t, text, campaignHTML)
	require.True(t, strings.HasSuffix(text, "\r\n"))
}

func TestBuildCampaignSMTPMessageRejectsInvalidSender(t *testing.T) {
	originalFrom := common.SMTPFrom
	common.SMTPFrom = "invalid"
	t.Cleanup(func() { common.SMTPFrom = originalFrom })

	_, err := buildCampaignSMTPMessage(campaignSubject, "member@example.com", campaignHTML)
	require.ErrorContains(t, err, "sender address is invalid")
}
