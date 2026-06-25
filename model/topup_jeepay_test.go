package model

import (
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/require"
)

func TestRechargeJeepayIsIdempotent(t *testing.T) {
	truncateTables(t)

	user := &User{
		Id:       701,
		Username: "jeepay_idempotent_user",
		Status:   common.UserStatusEnabled,
		Quota:    0,
	}
	require.NoError(t, DB.Create(user).Error)

	topUp := &TopUp{
		UserId:          user.Id,
		Amount:          100,
		Money:           12.34,
		TradeNo:         "jeepay-idempotent-001",
		PaymentMethod:   "jeepay_ali_cashier",
		PaymentProvider: PaymentProviderJeepay,
		Status:          common.TopUpStatusPending,
		CreateTime:      time.Now().Unix(),
	}
	require.NoError(t, topUp.Insert())

	payload := map[string]string{
		"mchOrderNo": topUp.TradeNo,
		"state":      "2",
		"amount":     "1234",
	}

	require.NoError(t, RechargeJeepay(topUp.TradeNo, payload, "127.0.0.1"))
	require.NoError(t, RechargeJeepay(topUp.TradeNo, payload, "127.0.0.1"))

	stored := GetTopUpByTradeNo(topUp.TradeNo)
	require.NotNil(t, stored)
	require.Equal(t, common.TopUpStatusSuccess, stored.Status)

	var refreshed User
	require.NoError(t, DB.Select("quota").Where("id = ?", user.Id).First(&refreshed).Error)
	require.Equal(t, int(topUp.Money*common.QuotaPerUnit), refreshed.Quota)
}
