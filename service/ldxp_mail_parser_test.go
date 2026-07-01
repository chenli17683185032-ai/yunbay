package service

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseLdxpOrderMail_UserSample(t *testing.T) {
	raw := `感谢购买商品0.1 元测试
实付0.10元
数量:1,
付款时间2026-06-28 03:37:42
单号:LD260628UZJ97P,
以下是您的购买内容:
9470548686742880`

	mail, err := ParseLdxpOrderMail(raw)
	require.NoError(t, err)
	assert.Equal(t, "0.1 元测试", mail.ProductName)
	assert.Equal(t, int64(10), mail.PaidCents)
	assert.Equal(t, 1, mail.Quantity)
	assert.Equal(t, "LD260628UZJ97P", mail.OrderNo)
	assert.Equal(t, int64(1782589062), mail.PaymentTime)
	assert.Contains(t, mail.ContentMasked, "9470********2880")
}

func TestParseLdxpOrderMail_Variants(t *testing.T) {
	raw := `感谢购买商品 云贝 10 元充值<br/>实付：10.30 元<br/>数量：1<br/>付款时间：2026-06-28 03:37:42<br/>单号：LD260628ABC123，<br/>以下是您的购买内容:<br/>abcdef1234567890`

	mail, err := ParseLdxpOrderMail(raw)
	require.NoError(t, err)
	assert.Equal(t, int64(1030), mail.PaidCents)
	assert.Equal(t, "LD260628ABC123", mail.OrderNo)
	assert.Contains(t, mail.ContentMasked, "abcd********7890")
}

func TestMoneyTextToCents(t *testing.T) {
	cases := map[string]int64{
		"0.10":   10,
		"10.3":   1030,
		"10.30元": 1030,
		"425":    42500,
	}
	for input, expected := range cases {
		actual, err := MoneyTextToCents(input)
		require.NoError(t, err, input)
		assert.Equal(t, expected, actual, input)
	}
}
