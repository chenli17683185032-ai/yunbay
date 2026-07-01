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
	assert.Equal(t, "9470********2880", mail.ContentMasked)
}

func TestParseLdxpOrderMail_Variants(t *testing.T) {
	raw := `感谢购买商品 云贝 10 元充值<br/>实付：10.30 元<br/>数量：1<br/>付款时间：2026-06-28 03:37:42<br/>单号：LD260628ABC123，<br/>以下是您的购买内容:<br/>abcdef1234567890`

	mail, err := ParseLdxpOrderMail(raw)
	require.NoError(t, err)
	assert.Equal(t, int64(1030), mail.PaidCents)
	assert.Equal(t, "LD260628ABC123", mail.OrderNo)
	assert.Equal(t, "abcd********7890", mail.ContentMasked)
}

func TestParseLdxpOrderMail_ContentStopsBeforeFooter(t *testing.T) {
	raw := `感谢购买商品0.1 元测试
实付0.10元
数量:1,
付款时间2026-06-28 03:37:42
单号:LD260628UZJ97P,
以下是您的购买内容:
9470548686742880

感谢使用 LDXP
如非本人操作请忽略此邮件`

	mail, err := ParseLdxpOrderMail(raw)
	require.NoError(t, err)
	assert.Equal(t, "9470********2880", mail.ContentMasked)
}

func TestParseLdxpOrderMail_HTMLEntityAndBlockTags(t *testing.T) {
	raw := `<p>感谢购买商品 云贝 10 元充值</p><p>实付：10.30&nbsp;元</p><p>数量：1</p><p>付款时间：2026-06-28 03:37:42</p><p>单号：LD260628ABC123&nbsp;</p><p>以下是您的购买内容:</p><p>abcdef1234567890</p>`

	mail, err := ParseLdxpOrderMail(raw)
	require.NoError(t, err)
	assert.Equal(t, "云贝 10 元充值", mail.ProductName)
	assert.Equal(t, int64(1030), mail.PaidCents)
	assert.Equal(t, "LD260628ABC123", mail.OrderNo)
	assert.Equal(t, "abcd********7890", mail.ContentMasked)
}

func TestParseLdxpOrderMail_MinimalRequiredFields(t *testing.T) {
	mail, err := ParseLdxpOrderMail("实付1元\n单号:LDMIN1")

	require.NoError(t, err)
	assert.Equal(t, int64(100), mail.PaidCents)
	assert.Equal(t, "LDMIN1", mail.OrderNo)
	assert.Equal(t, 0, mail.Quantity)
	assert.Equal(t, int64(0), mail.PaymentTime)
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

func TestMoneyTextToCentsRejectsMoreThanTwoDecimalPlaces(t *testing.T) {
	for _, input := range []string{"10.005元", "0.001"} {
		_, err := MoneyTextToCents(input)
		require.Error(t, err, input)
	}
}
