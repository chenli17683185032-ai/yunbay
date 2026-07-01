# Admin Order Management Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build an admin-only order management page that shows 7-day/30-day/custom revenue analytics, transaction detail rows, immediate mail verification for LDXP/小铺 orders, per-order affiliate attribution, and affiliate withdrawal handling.

**Architecture:** Add an additive backend order-management module around dedicated LDXP mail/session and affiliate ledger tables, expose admin APIs through `/api/order-management/admin`, and keep verification logic in pure service functions plus a small async in-process job runner. Add a `web/default` feature page that consumes those APIs with React Query, uses the existing layout and shadcn/Base UI components, and adds the sidebar/i18n wiring required for admins to reach it.

**Tech Stack:** Go 1.25/Gin/GORM/shopspring decimal, SQLite/MySQL/PostgreSQL-compatible migrations, React 19/TypeScript/Rsbuild/TanStack Router/TanStack Query/VChart/Base UI/Tailwind CSS, Bun for frontend commands.

---

## Ground Rules From Spec and Repository

- Work from `/Users/ethan/Documents/yunbay`.
- Do not modify or remove protected project identity, organization identity, copyright, license, module path, or attribution strings.
- Do not commit mailbox authorization codes, IMAP passwords, QQ tokens, Worker tokens, SMTP tokens, or production secrets.
- Use `common.Marshal`, `common.Unmarshal`, `common.DecodeJson`, and related wrappers for JSON marshal/unmarshal in Go business code.
- Keep database code compatible with SQLite, MySQL >= 5.7.8, and PostgreSQL >= 9.6. Prefer GORM and Go-side aggregation for daily chart data.
- Store money comparisons in cents or `decimal.Decimal`; never compare `float64` directly for mail verification.
- Frontend code in `web/default` uses Bun and `t('English key')` for UI text. Add translations for `en`, `zh`, `fr`, `ja`, `ru`, and `vi`.
- Current workspace audit found no existing LDXP model files beyond the spec. The implementation below adds additive compatibility models. If the execution worktree already contains production LDXP models with equivalent fields, stop before creating duplicates and map this plan’s service/API layer to those existing names.
- Existing unrelated modified files under `/Users/ethan/Documents/yunbay/infra/sub2api/backend/internal/server/middleware/` must not be staged or committed by this feature.

---

## File Structure

### Backend files

- Create `model/order_management.go`
  - LDXP session/mail event models.
  - Affiliate commission/withdrawal models.
  - Status constants and query helpers.
  - Daily analytics and affiliate summary aggregation using GORM reads plus Go-side grouping.
- Create `model/order_management_test.go`
  - Model-level tests for status constants, aggregation, affiliate withdrawal state transition, and cents-based amount behavior.
- Modify `model/main.go`
  - Add new models to both normal and fast `AutoMigrate` lists.
- Modify `model/task_cas_test.go`
  - Add new models to the shared in-memory test DB migration and cleanup list.
- Create `service/ldxp_mail_parser.go`
  - Parse plain text and HTML-ish LDXP/小铺 order mail into normalized fields.
  - Convert money text to integer cents with decimal rounding to 2 places.
  - Mask card/token content for table display.
- Create `service/ldxp_mail_parser_test.go`
  - Tests for the user-provided email sample and formatting variants.
- Create `service/order_mail_verifier.go`
  - Pure verification rules: worker order number equals mail order number, and session external paid cents equals mail paid cents.
  - No comparison against site amount.
- Create `service/order_mail_verifier_test.go`
  - Tests for `500 -> 425`, `10 -> 10.30`, amount mismatch, and order number mismatch.
- Create `service/order_mail_source.go`
  - `LdxpMailSource` interface and initial stored-event source.
  - Environment-backed config shape for a future IMAP adapter without committing secrets.
- Create `service/order_mail_check_job.go`
  - In-process job runner for single-order and batch immediate verification.
  - Job status lookup for the frontend.
- Create `service/order_mail_check_job_test.go`
  - Tests for single-order verification and bounded batch runs.
- Create `dto/order_management.go`
  - Request/response structs for analytics, orders, mail-check jobs, affiliate stats, withdrawals, and source orders.
- Create `controller/order_management.go`
  - Admin handlers for analytics, orders, immediate mail check, job status, affiliate stats, source orders, withdrawal paid/reject.
- Create `controller/order_management_test.go`
  - Handler-level tests for range parsing, invalid withdrawal action input, and API response shapes.
- Modify `controller/audit.go`
  - Add audit action templates for mail check and withdrawal paid/reject.
- Modify `router/api-router.go`
  - Register `/api/order-management/admin` with `middleware.AdminAuth()`.

### Frontend files

- Create `web/default/src/routes/_authenticated/order-management/index.tsx`
  - Admin guard and search schema.
- Create `web/default/src/features/order-management/index.tsx`
  - Page shell: title, range controls, top action buttons, analytics cards/chart, order details section, affiliate stats section.
- Create `web/default/src/features/order-management/api.ts`
  - API client functions for all admin endpoints.
- Create `web/default/src/features/order-management/types.ts`
  - TypeScript types matching `dto/order_management.go` JSON.
- Create `web/default/src/features/order-management/lib/format.ts`
  - `formatCny`, `formatRate`, `formatTime`, status label helpers.
- Create `web/default/src/features/order-management/lib/format.test.ts`
  - Bun tests for currency/rate/status formatting helpers.
- Create `web/default/src/features/order-management/components/range-toolbar.tsx`
  - 7-day / 30-day / custom range controls and batch action buttons.
- Create `web/default/src/features/order-management/components/order-analytics-cards.tsx`
  - KPI cards.
- Create `web/default/src/features/order-management/components/order-trend-chart.tsx`
  - VChart daily revenue/order trend.
- Create `web/default/src/features/order-management/components/order-details-table.tsx`
  - Large transaction detail table, row status, verify/recheck buttons.
- Create `web/default/src/features/order-management/components/mail-check-status-badge.tsx`
  - Status badge component.
- Create `web/default/src/features/order-management/components/affiliate-stats-section.tsx`
  - Affiliate KPI and withdrawal table.
- Create `web/default/src/features/order-management/components/withdrawal-actions.tsx`
  - Paid/reject actions with confirmation dialog and admin remark.
- Create `web/default/src/features/order-management/components/source-orders-drawer.tsx`
  - Source order details for affiliate users.
- Modify `web/default/src/hooks/sidebar-data-model.ts`
  - Add `Order Management` entry to the admin nav group.
- Modify `web/default/src/hooks/sidebar-data-model.test.ts`
  - Assert admin users see `/order-management` and ordinary users do not.
- Modify `web/default/src/hooks/use-sidebar-config.ts`
  - Add `order_management` module defaults and URL mapping.
- Modify `model/user.go` and `controller/user.go`
  - Add `order_management: true` to generated admin/root sidebar module defaults for newly created users.
- Modify `web/default/src/i18n/locales/{en,zh,fr,ja,ru,vi}.json`
  - Add all order management UI translations.

---

## Backend Data Model Contract

Use this additive table shape unless an execution worktree already contains production-equivalent models.

```go
const (
	MailCheckStatusNotRequired      = "not_required"
	MailCheckStatusPending          = "pending"
	MailCheckStatusWaitingMail      = "waiting_mail"
	MailCheckStatusChecking         = "checking"
	MailCheckStatusVerified         = "verified"
	MailCheckStatusOrderMismatch    = "order_mismatch"
	MailCheckStatusAmountMismatch   = "amount_mismatch"
	MailCheckStatusMailParseFailed  = "mail_parse_failed"
	MailCheckStatusMailFetchFailed  = "mail_fetch_failed"
	MailCheckStatusTimeout          = "timeout"
)

const (
	AffiliateCommissionStatusPending   = "pending"
	AffiliateCommissionStatusAvailable = "available"
	AffiliateCommissionStatusWithdrawn = "withdrawn"
	AffiliateCommissionStatusRejected  = "rejected"
)

const (
	AffiliateWithdrawalStatusPending  = "pending"
	AffiliateWithdrawalStatusPaid     = "paid"
	AffiliateWithdrawalStatusRejected = "rejected"
)

type LdxpTopupSession struct {
	Id                int    `json:"id"`
	SessionId         string `json:"session_id" gorm:"uniqueIndex;type:varchar(64)"`
	UserId            int    `json:"user_id" gorm:"index"`
	TopUpId           int    `json:"topup_id" gorm:"index;default:0"`
	TradeNo           string `json:"trade_no" gorm:"type:varchar(255);index"`
	SiteAmountCents   int64  `json:"site_amount_cents" gorm:"type:bigint;not null;default:0"`
	ExternalPaidCents int64  `json:"external_paid_cents" gorm:"type:bigint;not null;default:0"`
	WorkerOrderNo     string `json:"worker_order_no" gorm:"type:varchar(64);index"`
	WorkerAmountCents int64  `json:"worker_amount_cents" gorm:"type:bigint;not null;default:0"`
	MailOrderNo       string `json:"mail_order_no" gorm:"type:varchar(64);index"`
	MailAmountCents   int64  `json:"mail_amount_cents" gorm:"type:bigint;not null;default:0"`
	MailStatus        string `json:"mail_status" gorm:"type:varchar(32);index;default:'pending'"`
	MailEventId       int    `json:"mail_event_id" gorm:"index;default:0"`
	ErrorCode         string `json:"error_code" gorm:"type:varchar(64);default:''"`
	ErrorMessage      string `json:"error_message" gorm:"type:varchar(512);default:''"`
	CreatedTime       int64  `json:"created_time" gorm:"index"`
	PaidTime          int64  `json:"paid_time" gorm:"index;default:0"`
	VerifiedTime      int64  `json:"verified_time" gorm:"index;default:0"`
	UpdatedTime       int64  `json:"updated_time" gorm:"autoUpdateTime"`
}

type LdxpMailEvent struct {
	Id             int    `json:"id"`
	SourceAccount  string `json:"source_account" gorm:"type:varchar(128);index"`
	MessageId      string `json:"message_id" gorm:"type:varchar(255);index"`
	ImapUid        string `json:"imap_uid" gorm:"type:varchar(64);index"`
	RawHash        string `json:"raw_hash" gorm:"type:char(64);uniqueIndex"`
	Subject        string `json:"subject" gorm:"type:varchar(255);default:''"`
	FromAddress    string `json:"from_address" gorm:"type:varchar(255);default:''"`
	ProductName    string `json:"product_name" gorm:"type:varchar(255);default:''"`
	OrderNo        string `json:"order_no" gorm:"type:varchar(64);index"`
	PaidCents      int64  `json:"paid_cents" gorm:"type:bigint;not null;default:0"`
	Quantity       int    `json:"quantity" gorm:"type:int;not null;default:0"`
	PaymentTime    int64  `json:"payment_time" gorm:"index;default:0"`
	ContentMasked  string `json:"content_masked" gorm:"type:text"`
	ParseStatus    string `json:"parse_status" gorm:"type:varchar(32);index;default:'parsed'"`
	ParseError     string `json:"parse_error" gorm:"type:varchar(512);default:''"`
	CreatedTime    int64  `json:"created_time" gorm:"index"`
}

type AffiliateCommission struct {
	Id                int    `json:"id"`
	InviterUserId     int    `json:"inviter_user_id" gorm:"index"`
	InviteeUserId     int    `json:"invitee_user_id" gorm:"index"`
	TopUpId           int    `json:"topup_id" gorm:"index;default:0"`
	SessionId         string `json:"session_id" gorm:"type:varchar(64);index"`
	TradeNo           string `json:"trade_no" gorm:"type:varchar(255);index"`
	BaseMoneyCents    int64  `json:"base_money_cents" gorm:"type:bigint;not null;default:0"`
	RateBps           int    `json:"rate_bps" gorm:"type:int;not null;default:0"`
	CommissionCents   int64  `json:"commission_cents" gorm:"type:bigint;not null;default:0"`
	Status            string `json:"status" gorm:"type:varchar(32);index;default:'available'"`
	CreatedTime       int64  `json:"created_time" gorm:"index"`
	ConfirmedTime     int64  `json:"confirmed_time" gorm:"index;default:0"`
	WithdrawalId      int    `json:"withdrawal_id" gorm:"index;default:0"`
}

type AffiliateWithdrawal struct {
	Id            int    `json:"id"`
	WithdrawalId  string `json:"withdrawal_id" gorm:"uniqueIndex;type:varchar(64)"`
	UserId        int    `json:"user_id" gorm:"index"`
	AmountCents   int64  `json:"amount_cents" gorm:"type:bigint;not null;default:0"`
	Contact       string `json:"contact" gorm:"type:varchar(255);not null"`
	Remark        string `json:"remark" gorm:"type:varchar(512);default:''"`
	Status        string `json:"status" gorm:"type:varchar(32);index;default:'pending'"`
	AdminRemark   string `json:"admin_remark" gorm:"type:varchar(512);default:''"`
	CreatedTime   int64  `json:"created_time" gorm:"index"`
	ProcessedTime int64  `json:"processed_time" gorm:"index;default:0"`
	ProcessedBy   int    `json:"processed_by" gorm:"index;default:0"`
}
```

---

## Task 1: Create backend model foundation and migrations

**Files:**
- Create: `/Users/ethan/Documents/yunbay/model/order_management.go`
- Create: `/Users/ethan/Documents/yunbay/model/order_management_test.go`
- Modify: `/Users/ethan/Documents/yunbay/model/main.go`
- Modify: `/Users/ethan/Documents/yunbay/model/task_cas_test.go`

- [ ] **Step 1: Write model tests first**

Create `/Users/ethan/Documents/yunbay/model/order_management_test.go` with focused tests for the additive models and state transitions:

```go
package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOrderManagementModelsPersistCentsAndStatuses(t *testing.T) {
	truncateTables(t)

	session := &LdxpTopupSession{
		SessionId:         "ldxp_test_1",
		UserId:            1001,
		TradeNo:           "WAFFO_PANCAKE-1001-1",
		SiteAmountCents:   50000,
		ExternalPaidCents: 42500,
		WorkerOrderNo:     "LD260628UZJ97P",
		MailStatus:        MailCheckStatusPending,
		CreatedTime:       1782600000,
	}
	require.NoError(t, DB.Create(session).Error)

	var saved LdxpTopupSession
	require.NoError(t, DB.Where("session_id = ?", "ldxp_test_1").First(&saved).Error)
	assert.Equal(t, int64(50000), saved.SiteAmountCents)
	assert.Equal(t, int64(42500), saved.ExternalPaidCents)
	assert.Equal(t, MailCheckStatusPending, saved.MailStatus)
}

func TestAffiliateWithdrawalPaidIsSingleTransition(t *testing.T) {
	truncateTables(t)

	withdrawal := &AffiliateWithdrawal{
		WithdrawalId: "affw_test_1",
		UserId:       2001,
		AmountCents:  5000,
		Contact:      "支付宝：138****8888",
		Status:       AffiliateWithdrawalStatusPending,
		CreatedTime:  common.GetTimestamp(),
	}
	require.NoError(t, DB.Create(withdrawal).Error)

	updated, err := MarkAffiliateWithdrawalPaid(withdrawal.Id, 99, "已通过支付宝打款")
	require.NoError(t, err)
	assert.Equal(t, AffiliateWithdrawalStatusPaid, updated.Status)
	assert.Equal(t, 99, updated.ProcessedBy)
	assert.NotZero(t, updated.ProcessedTime)

	_, err = MarkAffiliateWithdrawalPaid(withdrawal.Id, 99, "重复点击")
	require.ErrorIs(t, err, ErrAffiliateWithdrawalAlreadyProcessed)
}
```

- [ ] **Step 2: Run the model tests and confirm the expected compile failure**

Run:

```bash
cd /Users/ethan/Documents/yunbay
go test ./model -run 'TestOrderManagementModels|TestAffiliateWithdrawal' -count=1
```

Expected: FAIL because `LdxpTopupSession`, `AffiliateWithdrawal`, status constants, and `MarkAffiliateWithdrawalPaid` do not exist yet.

- [ ] **Step 3: Add the model file with constants, structs, and withdrawal state helpers**

Create `/Users/ethan/Documents/yunbay/model/order_management.go` using the structs in “Backend Data Model Contract” and add these helper errors/functions:

```go
var (
	ErrAffiliateWithdrawalNotFound         = errors.New("affiliate withdrawal not found")
	ErrAffiliateWithdrawalAlreadyProcessed = errors.New("affiliate withdrawal already processed")
)

func MarkAffiliateWithdrawalPaid(id int, adminId int, adminRemark string) (*AffiliateWithdrawal, error) {
	return updateAffiliateWithdrawalStatus(id, adminId, adminRemark, AffiliateWithdrawalStatusPaid)
}

func RejectAffiliateWithdrawal(id int, adminId int, adminRemark string) (*AffiliateWithdrawal, error) {
	return updateAffiliateWithdrawalStatus(id, adminId, adminRemark, AffiliateWithdrawalStatusRejected)
}

func updateAffiliateWithdrawalStatus(id int, adminId int, adminRemark string, status string) (*AffiliateWithdrawal, error) {
	if id <= 0 {
		return nil, ErrAffiliateWithdrawalNotFound
	}
	withdrawal := &AffiliateWithdrawal{}
	err := DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Set("gorm:query_option", "FOR UPDATE").Where("id = ?", id).First(withdrawal).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrAffiliateWithdrawalNotFound
			}
			return err
		}
		if withdrawal.Status != AffiliateWithdrawalStatusPending {
			return ErrAffiliateWithdrawalAlreadyProcessed
		}
		withdrawal.Status = status
		withdrawal.AdminRemark = strings.TrimSpace(adminRemark)
		withdrawal.ProcessedBy = adminId
		withdrawal.ProcessedTime = common.GetTimestamp()
		return tx.Save(withdrawal).Error
	})
	if err != nil {
		return nil, err
	}
	return withdrawal, nil
}
```

Use imports:

```go
import (
	"errors"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)
```

- [ ] **Step 4: Add migrations to both migration paths**

In `/Users/ethan/Documents/yunbay/model/main.go`, add these models to the `DB.AutoMigrate(...)` call in `migrateDB()` immediately after `&TopUp{},`:

```go
		&LdxpTopupSession{},
		&LdxpMailEvent{},
		&AffiliateCommission{},
		&AffiliateWithdrawal{},
```

In the `migrations := []struct { ... }` list in `migrateDBFast()`, add:

```go
		{&LdxpTopupSession{}, "LdxpTopupSession"},
		{&LdxpMailEvent{}, "LdxpMailEvent"},
		{&AffiliateCommission{}, "AffiliateCommission"},
		{&AffiliateWithdrawal{}, "AffiliateWithdrawal"},
```

Place them immediately after the `TopUp` entry.

- [ ] **Step 5: Add the new models to the shared test database**

In `/Users/ethan/Documents/yunbay/model/task_cas_test.go`, add the four new model types to `db.AutoMigrate(...)` after `&TopUp{},`:

```go
		&LdxpTopupSession{},
		&LdxpMailEvent{},
		&AffiliateCommission{},
		&AffiliateWithdrawal{},
```

Also add cleanup statements inside `truncateTables`:

```go
		DB.Exec("DELETE FROM ldxp_topup_sessions")
		DB.Exec("DELETE FROM ldxp_mail_events")
		DB.Exec("DELETE FROM affiliate_commissions")
		DB.Exec("DELETE FROM affiliate_withdrawals")
```

- [ ] **Step 6: Run the model tests and commit the model foundation**

Run:

```bash
cd /Users/ethan/Documents/yunbay
go test ./model -run 'TestOrderManagementModels|TestAffiliateWithdrawal' -count=1
```

Expected: PASS.

Commit only the model/migration/test files:

```bash
git add model/order_management.go model/order_management_test.go model/main.go model/task_cas_test.go
git commit -m "feat: add order management data models"
```

---

## Task 2: Add LDXP mail parsing and pure verification logic

**Files:**
- Create: `/Users/ethan/Documents/yunbay/service/ldxp_mail_parser.go`
- Create: `/Users/ethan/Documents/yunbay/service/ldxp_mail_parser_test.go`
- Create: `/Users/ethan/Documents/yunbay/service/order_mail_verifier.go`
- Create: `/Users/ethan/Documents/yunbay/service/order_mail_verifier_test.go`

- [ ] **Step 1: Write mail parser tests from the real sample**

Create `/Users/ethan/Documents/yunbay/service/ldxp_mail_parser_test.go`:

```go
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
		"0.10":    10,
		"10.3":    1030,
		"10.30元": 1030,
		"425":     42500,
	}
	for input, expected := range cases {
		actual, err := MoneyTextToCents(input)
		require.NoError(t, err, input)
		assert.Equal(t, expected, actual, input)
	}
}
```

- [ ] **Step 2: Run parser tests and confirm expected failure**

Run:

```bash
cd /Users/ethan/Documents/yunbay
go test ./service -run 'TestParseLdxpOrderMail|TestMoneyTextToCents' -count=1
```

Expected: FAIL because parser functions do not exist.

- [ ] **Step 3: Implement `ParsedLdxpMail`, money normalization, masking, and parser**

Create `/Users/ethan/Documents/yunbay/service/ldxp_mail_parser.go` with:

```go
package service

import (
	"errors"
	"regexp"
	"strconv"
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
	ldxpPaidRe     = regexp.MustCompile(`实付\s*[:：]?\s*([0-9]+(?:\.[0-9]+)?)\s*元?`)
	ldxpQtyRe      = regexp.MustCompile(`数量\s*[:：]\s*([0-9]+)`) 
	ldxpTimeRe     = regexp.MustCompile(`付款时间\s*[:：]?\s*([0-9]{4}-[0-9]{2}-[0-9]{2}\s+[0-9]{2}:[0-9]{2}:[0-9]{2})`)
	ldxpOrderRe    = regexp.MustCompile(`单号\s*[:：]\s*([A-Za-z0-9_-]+)`)
	ldxpProductRe  = regexp.MustCompile(`感谢购买商品\s*(.+?)\s*(?:\n|实付|数量|付款时间|单号|$)`)
	ldxpContentRe  = regexp.MustCompile(`以下是您的购买内容\s*[:：]?\s*(.+)$`)
	htmlBreakRe     = regexp.MustCompile(`(?i)<br\s*/?>`)
	htmlTagRe       = regexp.MustCompile(`<[^>]+>`)
)

func MoneyTextToCents(input string) (int64, error) {
	cleaned := strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(input), "元"))
	if cleaned == "" {
		return 0, errors.New("money text is empty")
	}
	value, err := decimal.NewFromString(cleaned)
	if err != nil {
		return 0, err
	}
	return value.Mul(decimal.NewFromInt(100)).Round(0).IntPart(), nil
}

func ParseLdxpOrderMail(raw string) (*ParsedLdxpMail, error) {
	text := normalizeMailText(raw)
	paidMatch := ldxpPaidRe.FindStringSubmatch(text)
	orderMatch := ldxpOrderRe.FindStringSubmatch(text)
	if len(paidMatch) < 2 || len(orderMatch) < 2 {
		return nil, errors.New("missing paid amount or order number")
	}
	paidCents, err := MoneyTextToCents(paidMatch[1])
	if err != nil {
		return nil, err
	}

	quantity := 0
	if qtyMatch := ldxpQtyRe.FindStringSubmatch(text); len(qtyMatch) >= 2 {
		quantity, _ = strconv.Atoi(qtyMatch[1])
	}

	paymentTime := int64(0)
	if timeMatch := ldxpTimeRe.FindStringSubmatch(text); len(timeMatch) >= 2 {
		if parsed, err := time.ParseInLocation("2006-01-02 15:04:05", timeMatch[1], time.FixedZone("Asia/Shanghai", 8*60*60)); err == nil {
			paymentTime = parsed.Unix()
		}
	}

	productName := ""
	if productMatch := ldxpProductRe.FindStringSubmatch(text); len(productMatch) >= 2 {
		productName = strings.TrimSpace(strings.Trim(productMatch[1], "，, "))
	}

	content := ""
	if contentMatch := ldxpContentRe.FindStringSubmatch(text); len(contentMatch) >= 2 {
		content = strings.TrimSpace(contentMatch[1])
	}

	return &ParsedLdxpMail{
		ProductName:   productName,
		PaidCents:     paidCents,
		Quantity:      quantity,
		PaymentTime:   paymentTime,
		OrderNo:       strings.TrimSpace(orderMatch[1]),
		ContentMasked: maskPurchaseContent(content),
	}, nil
}

func normalizeMailText(raw string) string {
	text := htmlBreakRe.ReplaceAllString(raw, "\n")
	text = htmlTagRe.ReplaceAllString(text, "")
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	return strings.TrimSpace(text)
}

func maskPurchaseContent(content string) string {
	content = strings.TrimSpace(content)
	if len([]rune(content)) <= 8 {
		return content
	}
	runes := []rune(content)
	return string(runes[:4]) + "********" + string(runes[len(runes)-4:])
}
```

- [ ] **Step 4: Write pure verification tests**

Create `/Users/ethan/Documents/yunbay/service/order_mail_verifier_test.go`:

```go
package service

import (
	"testing"

	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/assert"
)

func TestVerifyLdxpMailUsesExternalPaidAmount(t *testing.T) {
	session := &model.LdxpTopupSession{
		SiteAmountCents:   50000,
		ExternalPaidCents: 42500,
		WorkerOrderNo:     "LD260628UZJ97P",
	}
	mail := &ParsedLdxpMail{PaidCents: 42500, OrderNo: "LD260628UZJ97P"}

	result := VerifyLdxpMail(session, mail)
	assert.Equal(t, model.MailCheckStatusVerified, result.Status)
}

func TestVerifyLdxpMailAllowsUserPaidFeeOnTop(t *testing.T) {
	session := &model.LdxpTopupSession{
		SiteAmountCents:   1000,
		ExternalPaidCents: 1030,
		WorkerOrderNo:     "LD260628UZJ97P",
	}
	mail := &ParsedLdxpMail{PaidCents: 1030, OrderNo: "LD260628UZJ97P"}

	result := VerifyLdxpMail(session, mail)
	assert.Equal(t, model.MailCheckStatusVerified, result.Status)
}

func TestVerifyLdxpMailRejectsAmountMismatch(t *testing.T) {
	session := &model.LdxpTopupSession{
		SiteAmountCents:   1000,
		ExternalPaidCents: 1030,
		WorkerOrderNo:     "LD260628UZJ97P",
	}
	mail := &ParsedLdxpMail{PaidCents: 1000, OrderNo: "LD260628UZJ97P"}

	result := VerifyLdxpMail(session, mail)
	assert.Equal(t, model.MailCheckStatusAmountMismatch, result.Status)
	assert.Equal(t, "amount_mismatch", result.ErrorCode)
}

func TestVerifyLdxpMailRejectsOrderMismatch(t *testing.T) {
	session := &model.LdxpTopupSession{
		ExternalPaidCents: 1030,
		WorkerOrderNo:     "LD260628UZJ97P",
	}
	mail := &ParsedLdxpMail{PaidCents: 1030, OrderNo: "LD260628OTHER"}

	result := VerifyLdxpMail(session, mail)
	assert.Equal(t, model.MailCheckStatusOrderMismatch, result.Status)
	assert.Equal(t, "order_mismatch", result.ErrorCode)
}
```

- [ ] **Step 5: Implement pure verification**

Create `/Users/ethan/Documents/yunbay/service/order_mail_verifier.go`:

```go
package service

import (
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/model"
)

type MailVerificationResult struct {
	Status       string
	ErrorCode    string
	ErrorMessage string
}

func VerifyLdxpMail(session *model.LdxpTopupSession, mail *ParsedLdxpMail) MailVerificationResult {
	if session == nil || mail == nil {
		return MailVerificationResult{Status: model.MailCheckStatusMailParseFailed, ErrorCode: "missing_data", ErrorMessage: "订单或邮件数据缺失"}
	}
	workerOrderNo := strings.TrimSpace(session.WorkerOrderNo)
	mailOrderNo := strings.TrimSpace(mail.OrderNo)
	if workerOrderNo == "" || mailOrderNo == "" || workerOrderNo != mailOrderNo {
		return MailVerificationResult{Status: model.MailCheckStatusOrderMismatch, ErrorCode: "order_mismatch", ErrorMessage: fmt.Sprintf("小铺单号不一致：订单 %s，邮件 %s", workerOrderNo, mailOrderNo)}
	}
	if session.ExternalPaidCents != mail.PaidCents {
		return MailVerificationResult{Status: model.MailCheckStatusAmountMismatch, ErrorCode: "amount_mismatch", ErrorMessage: fmt.Sprintf("实付金额不一致：订单 %.2f，邮件 %.2f", float64(session.ExternalPaidCents)/100, float64(mail.PaidCents)/100)}
	}
	return MailVerificationResult{Status: model.MailCheckStatusVerified}
}
```

- [ ] **Step 6: Run parser and verifier tests, then commit**

Run:

```bash
cd /Users/ethan/Documents/yunbay
go test ./service -run 'TestParseLdxpOrderMail|TestMoneyTextToCents|TestVerifyLdxpMail' -count=1
```

Expected: PASS.

Commit:

```bash
git add service/ldxp_mail_parser.go service/ldxp_mail_parser_test.go service/order_mail_verifier.go service/order_mail_verifier_test.go
git commit -m "feat: add LDXP mail parser and verifier"
```

---

## Task 3: Add backend query helpers for analytics, orders, and affiliate stats

**Files:**
- Modify: `/Users/ethan/Documents/yunbay/model/order_management.go`
- Modify: `/Users/ethan/Documents/yunbay/model/order_management_test.go`

- [ ] **Step 1: Add query helper tests**

Append these tests to `/Users/ethan/Documents/yunbay/model/order_management_test.go`:

```go
func TestOrderManagementAnalyticsAggregatesByDayInGo(t *testing.T) {
	truncateTables(t)

	records := []*LdxpTopupSession{
		{SessionId: "s1", UserId: 1, SiteAmountCents: 1000, ExternalPaidCents: 1030, MailStatus: MailCheckStatusVerified, CreatedTime: 1782518400},
		{SessionId: "s2", UserId: 2, SiteAmountCents: 50000, ExternalPaidCents: 42500, MailStatus: MailCheckStatusAmountMismatch, CreatedTime: 1782518500},
		{SessionId: "s3", UserId: 3, SiteAmountCents: 2000, ExternalPaidCents: 2060, MailStatus: MailCheckStatusPending, CreatedTime: 1782604800},
	}
	for _, record := range records {
		require.NoError(t, DB.Create(record).Error)
	}

	result, err := GetOrderManagementAnalytics(1782518400, 1782691199)
	require.NoError(t, err)
	assert.Equal(t, int64(53000), result.Summary.SiteAmountCents)
	assert.Equal(t, int64(45590), result.Summary.ExternalPaidCents)
	assert.Equal(t, 3, result.Summary.OrderCount)
	assert.Equal(t, 1, result.Summary.MailVerifiedCount)
	assert.Equal(t, 1, result.Summary.MailErrorCount)
	assert.Len(t, result.Daily, 2)
	assert.Equal(t, "2026-06-27", result.Daily[0].Date)
}

func TestOrderManagementAffiliateStatsIncludesWithdrawalInfo(t *testing.T) {
	truncateTables(t)

	require.NoError(t, DB.Create(&User{Id: 77, Username: "inviter", Status: common.UserStatusEnabled}).Error)
	require.NoError(t, DB.Create(&AffiliateCommission{InviterUserId: 77, InviteeUserId: 88, TradeNo: "trade1", BaseMoneyCents: 42500, RateBps: 1500, CommissionCents: 6375, Status: AffiliateCommissionStatusAvailable, CreatedTime: 1782518400}).Error)
	require.NoError(t, DB.Create(&AffiliateWithdrawal{WithdrawalId: "affw_pending", UserId: 77, AmountCents: 5000, Contact: "支付宝：138****8888", Status: AffiliateWithdrawalStatusPending, CreatedTime: 1782600000}).Error)

	result, err := GetAffiliateStats(1782518400, 1782691199, "pending", 0, 20)
	require.NoError(t, err)
	assert.Equal(t, 1, result.Summary.AffiliateUserCount)
	assert.Equal(t, int64(6375), result.Summary.PeriodCommissionCents)
	assert.Equal(t, 1, result.Summary.PendingWithdrawalUserCount)
	assert.Len(t, result.Items, 1)
	assert.NotNil(t, result.Items[0].Withdrawal)
	assert.Equal(t, "支付宝：138****8888", result.Items[0].Withdrawal.Contact)
}
```

- [ ] **Step 2: Run query helper tests and confirm expected failure**

Run:

```bash
cd /Users/ethan/Documents/yunbay
go test ./model -run 'TestOrderManagementAnalytics|TestOrderManagementAffiliateStats' -count=1
```

Expected: FAIL because query helper types/functions do not exist yet.

- [ ] **Step 3: Add query DTOs inside the model package**

Append these model-package result structs to `/Users/ethan/Documents/yunbay/model/order_management.go`:

```go
type OrderAnalyticsSummary struct {
	SiteAmountCents     int64
	ExternalPaidCents   int64
	OrderCount          int
	MailVerifiedCount   int
	MailPendingCount    int
	MailErrorCount      int
	MailVerifiedRate    float64
	AffiliateUserCount  int
	AffiliateAmountCents int64
	PendingWithdrawalCount int
	PendingWithdrawalCents int64
}

type OrderAnalyticsDaily struct {
	Date              string
	SiteAmountCents   int64
	ExternalPaidCents int64
	OrderCount        int
	MailVerifiedCount int
	MailErrorCount    int
}

type OrderAnalyticsResult struct {
	Summary OrderAnalyticsSummary
	Daily   []OrderAnalyticsDaily
}

type AffiliateStatsSummary struct {
	AffiliateUserCount                 int
	PeriodCommissionCents              int64
	PendingWithdrawalUserCount         int
	PendingWithdrawalCents             int64
	AvailableWithoutWithdrawalUserCount int
}

type AffiliateWithdrawalInfo struct {
	Id            int
	WithdrawalId  string
	AmountCents   int64
	Contact       string
	Remark        string
	Status        string
	CreatedTime   int64
	AdminRemark   string
	ProcessedTime int64
}

type AffiliateStatsItem struct {
	UserId                int
	Username              string
	PeriodCommissionCents int64
	TotalCommissionCents  int64
	AvailableCents        int64
	WithdrawnCents        int64
	Withdrawal            *AffiliateWithdrawalInfo
}

type AffiliateSourceOrderRow struct {
	OrderTime        int64
	InviteeUserId    int
	InviteeUsername  string
	TradeNo          string
	WorkerOrderNo    string
	BaseMoneyCents   int64
	RateBps          int
	CommissionCents  int64
	MailStatus       string
}

type OrderManagementOrderRow struct {
	Id                    int
	SessionId             string
	UserId                int
	Username              string
	SiteAmountCents       int64
	ExternalPaidCents     int64
	WorkerOrderNo         string
	MailOrderNo           string
	MailAmountCents       int64
	MailStatus            string
	ErrorCode             string
	ErrorMessage          string
	CreatedTime           int64
	VerifiedTime          int64
	AffiliateInviterId    int
	AffiliateCommissionCents int64
	AffiliateStatus       string
}
```

- [ ] **Step 4: Implement analytics aggregation in Go**

Add `GetOrderManagementAnalytics(startTime, endTime int64) (*OrderAnalyticsResult, error)` to `/Users/ethan/Documents/yunbay/model/order_management.go`:

```go
func GetOrderManagementAnalytics(startTime, endTime int64) (*OrderAnalyticsResult, error) {
	var sessions []LdxpTopupSession
	if err := DB.Where("created_time >= ? AND created_time <= ?", startTime, endTime).Order("created_time asc").Find(&sessions).Error; err != nil {
		return nil, err
	}
	result := &OrderAnalyticsResult{}
	dailyMap := map[string]*OrderAnalyticsDaily{}
	for _, session := range sessions {
		result.Summary.SiteAmountCents += session.SiteAmountCents
		result.Summary.ExternalPaidCents += session.ExternalPaidCents
		result.Summary.OrderCount++
		switch session.MailStatus {
		case MailCheckStatusVerified:
			result.Summary.MailVerifiedCount++
		case MailCheckStatusAmountMismatch, MailCheckStatusOrderMismatch, MailCheckStatusMailParseFailed, MailCheckStatusMailFetchFailed, MailCheckStatusTimeout:
			result.Summary.MailErrorCount++
		default:
			result.Summary.MailPendingCount++
		}

		dateKey := time.Unix(session.CreatedTime, 0).Format("2006-01-02")
		daily := dailyMap[dateKey]
		if daily == nil {
			daily = &OrderAnalyticsDaily{Date: dateKey}
			dailyMap[dateKey] = daily
		}
		daily.SiteAmountCents += session.SiteAmountCents
		daily.ExternalPaidCents += session.ExternalPaidCents
		daily.OrderCount++
		if session.MailStatus == MailCheckStatusVerified {
			daily.MailVerifiedCount++
		}
		if session.MailStatus == MailCheckStatusAmountMismatch || session.MailStatus == MailCheckStatusOrderMismatch || session.MailStatus == MailCheckStatusMailParseFailed || session.MailStatus == MailCheckStatusMailFetchFailed || session.MailStatus == MailCheckStatusTimeout {
			daily.MailErrorCount++
		}
	}
	if result.Summary.OrderCount > 0 {
		result.Summary.MailVerifiedRate = float64(result.Summary.MailVerifiedCount) / float64(result.Summary.OrderCount)
	}
	keys := make([]string, 0, len(dailyMap))
	for key := range dailyMap {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		result.Daily = append(result.Daily, *dailyMap[key])
	}
	return result, nil
}
```

Add imports if missing:

```go
import (
	"sort"
	"time"
)
```

Merge with existing imports rather than creating a second import block.

- [ ] **Step 5: Implement affiliate stats queries**

Add `GetAffiliateStats(startTime, endTime int64, withdrawalStatus string, offset int, limit int) (*AffiliateStatsResult, error)` to `/Users/ethan/Documents/yunbay/model/order_management.go`. Use GORM `.Find` reads and Go maps:

```go
func GetAffiliateStats(startTime, endTime int64, withdrawalStatus string, offset int, limit int) (*AffiliateStatsResult, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	var commissions []AffiliateCommission
	if err := DB.Where("created_time >= ? AND created_time <= ?", startTime, endTime).Find(&commissions).Error; err != nil {
		return nil, err
	}
	periodByUser := map[int]int64{}
	usersSet := map[int]struct{}{}
	for _, commission := range commissions {
		periodByUser[commission.InviterUserId] += commission.CommissionCents
		usersSet[commission.InviterUserId] = struct{}{}
	}

	var allCommissions []AffiliateCommission
	if err := DB.Find(&allCommissions).Error; err != nil {
		return nil, err
	}
	totalByUser := map[int]int64{}
	availableByUser := map[int]int64{}
	withdrawnByUser := map[int]int64{}
	for _, commission := range allCommissions {
		totalByUser[commission.InviterUserId] += commission.CommissionCents
		if commission.Status == AffiliateCommissionStatusAvailable {
			availableByUser[commission.InviterUserId] += commission.CommissionCents
		}
		if commission.Status == AffiliateCommissionStatusWithdrawn {
			withdrawnByUser[commission.InviterUserId] += commission.CommissionCents
		}
		usersSet[commission.InviterUserId] = struct{}{}
	}

	var withdrawals []AffiliateWithdrawal
	withdrawalQuery := DB.Order("created_time desc")
	if withdrawalStatus != "" {
		withdrawalQuery = withdrawalQuery.Where("status = ?", withdrawalStatus)
	}
	if err := withdrawalQuery.Find(&withdrawals).Error; err != nil {
		return nil, err
	}
	latestWithdrawalByUser := map[int]AffiliateWithdrawal{}
	pendingWithdrawalUsers := map[int]struct{}{}
	for _, withdrawal := range withdrawals {
		if _, exists := latestWithdrawalByUser[withdrawal.UserId]; !exists {
			latestWithdrawalByUser[withdrawal.UserId] = withdrawal
		}
		if withdrawal.Status == AffiliateWithdrawalStatusPending {
			pendingWithdrawalUsers[withdrawal.UserId] = struct{}{}
		}
	}

	userIds := make([]int, 0, len(usersSet))
	for userId := range usersSet {
		userIds = append(userIds, userId)
	}
	sort.Ints(userIds)
	result := &AffiliateStatsResult{Total: int64(len(userIds))}
	for _, userId := range userIds {
		result.Summary.PeriodCommissionCents += periodByUser[userId]
	}
	result.Summary.AffiliateUserCount = len(userIds)
	for userId, withdrawal := range latestWithdrawalByUser {
		if withdrawal.Status == AffiliateWithdrawalStatusPending {
			result.Summary.PendingWithdrawalCents += withdrawal.AmountCents
			_ = userId
		}
	}
	result.Summary.PendingWithdrawalUserCount = len(pendingWithdrawalUsers)
	for _, userId := range userIds {
		if availableByUser[userId] > 0 {
			if _, hasPending := pendingWithdrawalUsers[userId]; !hasPending {
				result.Summary.AvailableWithoutWithdrawalUserCount++
			}
		}
	}

	end := offset + limit
	if offset > len(userIds) {
		offset = len(userIds)
	}
	if end > len(userIds) {
		end = len(userIds)
	}
	pageUserIds := userIds[offset:end]
	usernameById := map[int]string{}
	if len(pageUserIds) > 0 {
		var users []User
		if err := DB.Select("id", "username").Where("id IN ?", pageUserIds).Find(&users).Error; err != nil {
			return nil, err
		}
		for _, user := range users {
			usernameById[user.Id] = user.Username
		}
	}
	for _, userId := range pageUserIds {
		item := AffiliateStatsItem{
			UserId:                userId,
			Username:              usernameById[userId],
			PeriodCommissionCents: periodByUser[userId],
			TotalCommissionCents:  totalByUser[userId],
			AvailableCents:        availableByUser[userId],
			WithdrawnCents:        withdrawnByUser[userId],
		}
		if withdrawal, ok := latestWithdrawalByUser[userId]; ok {
			item.Withdrawal = &AffiliateWithdrawalInfo{Id: withdrawal.Id, WithdrawalId: withdrawal.WithdrawalId, AmountCents: withdrawal.AmountCents, Contact: withdrawal.Contact, Remark: withdrawal.Remark, Status: withdrawal.Status, CreatedTime: withdrawal.CreatedTime, AdminRemark: withdrawal.AdminRemark, ProcessedTime: withdrawal.ProcessedTime}
		}
		result.Items = append(result.Items, item)
	}
	return result, nil
}
```

- [ ] **Step 6: Run model query tests and commit**

Run:

```bash
cd /Users/ethan/Documents/yunbay
go test ./model -run 'TestOrderManagementAnalytics|TestOrderManagementAffiliateStats|TestAffiliateWithdrawal|TestOrderManagementModels' -count=1
```

Expected: PASS.

Commit:

```bash
git add model/order_management.go model/order_management_test.go
git commit -m "feat: add order management query helpers"
```

---

## Task 4: Add mail-check job runner and stored mail source

**Files:**
- Create: `/Users/ethan/Documents/yunbay/service/order_mail_source.go`
- Create: `/Users/ethan/Documents/yunbay/service/order_mail_check_job.go`
- Create: `/Users/ethan/Documents/yunbay/service/order_mail_check_job_test.go`
- Modify: `/Users/ethan/Documents/yunbay/model/order_management.go`

- [ ] **Step 1: Add test helpers for job runner behavior**

Create `/Users/ethan/Documents/yunbay/service/order_mail_check_job_test.go`:

```go
package service

import (
	"context"
	"testing"

	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeMailSource struct {
	mails []*model.LdxpMailEvent
	err   error
}

func (f fakeMailSource) FetchRecent(ctx context.Context) ([]*model.LdxpMailEvent, error) {
	return f.mails, f.err
}

func TestRunSingleMailCheckVerifiesMatchingOrder(t *testing.T) {
	model.TruncateOrderManagementTablesForTest(t)

	session := &model.LdxpTopupSession{SessionId: "job_s1", UserId: 1, SiteAmountCents: 1000, ExternalPaidCents: 1030, WorkerOrderNo: "LD260628UZJ97P", MailStatus: model.MailCheckStatusPending, CreatedTime: 1782600000}
	require.NoError(t, model.DB.Create(session).Error)
	mail := &model.LdxpMailEvent{OrderNo: "LD260628UZJ97P", PaidCents: 1030, ParseStatus: "parsed", CreatedTime: 1782600001}
	require.NoError(t, model.DB.Create(mail).Error)

	job := NewOrderMailCheckRunner(fakeMailSource{mails: []*model.LdxpMailEvent{mail}})
	result := job.RunSingle(context.Background(), session.Id)
	require.NoError(t, result.Error)
	assert.Equal(t, 1, result.AffectedCount)

	var saved model.LdxpTopupSession
	require.NoError(t, model.DB.First(&saved, session.Id).Error)
	assert.Equal(t, model.MailCheckStatusVerified, saved.MailStatus)
	assert.Equal(t, int64(1030), saved.MailAmountCents)
}

func TestRunBatchMailCheckHonorsLimit(t *testing.T) {
	model.TruncateOrderManagementTablesForTest(t)
	for i := 0; i < 3; i++ {
		require.NoError(t, model.DB.Create(&model.LdxpTopupSession{SessionId: "batch_s" + string(rune('1'+i)), UserId: 1, ExternalPaidCents: 100, WorkerOrderNo: "LD260628B" + string(rune('1'+i)), MailStatus: model.MailCheckStatusPending, CreatedTime: 1782600000 + int64(i)}).Error)
	}
	job := NewOrderMailCheckRunner(fakeMailSource{})
	result := job.RunBatch(context.Background(), model.OrderMailCheckBatchFilter{StartTime: 1782600000, EndTime: 1782609999, Limit: 2})
	require.NoError(t, result.Error)
	assert.Equal(t, 2, result.AffectedCount)
}
```

- [ ] **Step 2: Add test-only cleanup helper in model**

Append this helper to `/Users/ethan/Documents/yunbay/model/order_management.go`:

```go
func TruncateOrderManagementTablesForTest(t interface{ Helper(); Cleanup(func()) }) {
	t.Helper()
	t.Cleanup(func() {
		DB.Exec("DELETE FROM ldxp_topup_sessions")
		DB.Exec("DELETE FROM ldxp_mail_events")
		DB.Exec("DELETE FROM affiliate_commissions")
		DB.Exec("DELETE FROM affiliate_withdrawals")
	})
}
```

This helper avoids importing unexported `truncateTables` from model tests into the service package.

- [ ] **Step 3: Run job tests and confirm expected failure**

Run:

```bash
cd /Users/ethan/Documents/yunbay
go test ./service -run 'TestRunSingleMailCheck|TestRunBatchMailCheck' -count=1
```

Expected: FAIL because `LdxpMailSource`, `NewOrderMailCheckRunner`, and batch filter types do not exist yet.

- [ ] **Step 4: Add mail source interface and stored-event source**

Create `/Users/ethan/Documents/yunbay/service/order_mail_source.go`:

```go
package service

import (
	"context"

	"github.com/QuantumNous/new-api/model"
)

type LdxpMailSource interface {
	FetchRecent(ctx context.Context) ([]*model.LdxpMailEvent, error)
}

type StoredLdxpMailSource struct{}

func (StoredLdxpMailSource) FetchRecent(ctx context.Context) ([]*model.LdxpMailEvent, error) {
	var events []*model.LdxpMailEvent
	err := model.DB.WithContext(ctx).Where("parse_status = ?", "parsed").Order("created_time desc").Limit(500).Find(&events).Error
	return events, err
}
```

- [ ] **Step 5: Add batch filter types to model**

Append to `/Users/ethan/Documents/yunbay/model/order_management.go`:

```go
type OrderMailCheckBatchFilter struct {
	StartTime int64
	EndTime   int64
	Limit     int
}

func ListMailCheckCandidates(filter OrderMailCheckBatchFilter) ([]LdxpTopupSession, error) {
	limit := filter.Limit
	if limit <= 0 || limit > 100 {
		limit = 100
	}
	query := DB.Where("mail_status IN ?", []string{MailCheckStatusPending, MailCheckStatusWaitingMail, MailCheckStatusAmountMismatch, MailCheckStatusOrderMismatch, MailCheckStatusMailFetchFailed, MailCheckStatusMailParseFailed, MailCheckStatusTimeout})
	if filter.StartTime > 0 {
		query = query.Where("created_time >= ?", filter.StartTime)
	}
	if filter.EndTime > 0 {
		query = query.Where("created_time <= ?", filter.EndTime)
	}
	var sessions []LdxpTopupSession
	err := query.Order("created_time desc").Limit(limit).Find(&sessions).Error
	return sessions, err
}
```

- [ ] **Step 6: Implement the job runner**

Create `/Users/ethan/Documents/yunbay/service/order_mail_check_job.go`:

```go
package service

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
)

type OrderMailCheckResult struct {
	JobId         string
	AffectedCount int
	Error         error
}

type OrderMailCheckJobStatus struct {
	JobId         string `json:"job_id"`
	Status        string `json:"status"`
	AffectedCount int    `json:"affected_count"`
	ErrorMessage   string `json:"error_message"`
	CreatedTime    int64  `json:"created_time"`
	FinishedTime   int64  `json:"finished_time"`
}

type OrderMailCheckRunner struct {
	source LdxpMailSource
	mu     sync.Mutex
	jobs   map[string]*OrderMailCheckJobStatus
}

func NewOrderMailCheckRunner(source LdxpMailSource) *OrderMailCheckRunner {
	if source == nil {
		source = StoredLdxpMailSource{}
	}
	return &OrderMailCheckRunner{source: source, jobs: map[string]*OrderMailCheckJobStatus{}}
}

var defaultOrderMailCheckRunner = NewOrderMailCheckRunner(StoredLdxpMailSource{})

func DefaultOrderMailCheckRunner() *OrderMailCheckRunner {
	return defaultOrderMailCheckRunner
}

func (r *OrderMailCheckRunner) StartSingle(ctx context.Context, sessionId int) *OrderMailCheckJobStatus {
	job := r.createJob()
	go func() {
		result := r.RunSingle(ctx, sessionId)
		r.finishJob(job.JobId, result)
	}()
	return job
}

func (r *OrderMailCheckRunner) StartBatch(ctx context.Context, filter model.OrderMailCheckBatchFilter) *OrderMailCheckJobStatus {
	job := r.createJob()
	go func() {
		result := r.RunBatch(ctx, filter)
		r.finishJob(job.JobId, result)
	}()
	return job
}

func (r *OrderMailCheckRunner) GetJob(jobId string) (*OrderMailCheckJobStatus, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	job, ok := r.jobs[jobId]
	if !ok {
		return nil, false
	}
	copy := *job
	return &copy, true
}

func (r *OrderMailCheckRunner) RunSingle(ctx context.Context, sessionId int) OrderMailCheckResult {
	var session model.LdxpTopupSession
	if err := model.DB.WithContext(ctx).First(&session, sessionId).Error; err != nil {
		return OrderMailCheckResult{AffectedCount: 0, Error: err}
	}
	return r.verifySessions(ctx, []model.LdxpTopupSession{session})
}

func (r *OrderMailCheckRunner) RunBatch(ctx context.Context, filter model.OrderMailCheckBatchFilter) OrderMailCheckResult {
	sessions, err := model.ListMailCheckCandidates(filter)
	if err != nil {
		return OrderMailCheckResult{Error: err}
	}
	return r.verifySessions(ctx, sessions)
}

func (r *OrderMailCheckRunner) verifySessions(ctx context.Context, sessions []model.LdxpTopupSession) OrderMailCheckResult {
	if len(sessions) == 0 {
		return OrderMailCheckResult{}
	}
	mails, err := r.source.FetchRecent(ctx)
	if err != nil {
		for _, session := range sessions {
			model.DB.Model(&model.LdxpTopupSession{}).Where("id = ?", session.Id).Updates(map[string]interface{}{"mail_status": model.MailCheckStatusMailFetchFailed, "error_code": "mail_fetch_failed", "error_message": err.Error()})
		}
		return OrderMailCheckResult{AffectedCount: len(sessions), Error: err}
	}
	mailByOrder := map[string]*model.LdxpMailEvent{}
	for _, mail := range mails {
		if mail != nil && mail.OrderNo != "" {
			mailByOrder[mail.OrderNo] = mail
		}
	}
	affected := 0
	for _, session := range sessions {
		affected++
		model.DB.Model(&model.LdxpTopupSession{}).Where("id = ?", session.Id).Update("mail_status", model.MailCheckStatusChecking)
		mailEvent := mailByOrder[session.WorkerOrderNo]
		if mailEvent == nil {
			model.DB.Model(&model.LdxpTopupSession{}).Where("id = ?", session.Id).Updates(map[string]interface{}{"mail_status": model.MailCheckStatusWaitingMail, "error_code": "waiting_mail", "error_message": "未找到匹配订单确认邮件"})
			continue
		}
		parsed := &ParsedLdxpMail{OrderNo: mailEvent.OrderNo, PaidCents: mailEvent.PaidCents, PaymentTime: mailEvent.PaymentTime, Quantity: mailEvent.Quantity, ProductName: mailEvent.ProductName, ContentMasked: mailEvent.ContentMasked}
		result := VerifyLdxpMail(&session, parsed)
		updates := map[string]interface{}{"mail_order_no": mailEvent.OrderNo, "mail_amount_cents": mailEvent.PaidCents, "mail_event_id": mailEvent.Id, "mail_status": result.Status, "error_code": result.ErrorCode, "error_message": result.ErrorMessage}
		if result.Status == model.MailCheckStatusVerified {
			updates["verified_time"] = common.GetTimestamp()
		}
		model.DB.Model(&model.LdxpTopupSession{}).Where("id = ?", session.Id).Updates(updates)
	}
	return OrderMailCheckResult{AffectedCount: affected}
}

func (r *OrderMailCheckRunner) createJob() *OrderMailCheckJobStatus {
	job := &OrderMailCheckJobStatus{JobId: fmt.Sprintf("mailcheck_%d", common.GetTimestampNano()), Status: "running", CreatedTime: common.GetTimestamp()}
	r.mu.Lock()
	r.jobs[job.JobId] = job
	r.mu.Unlock()
	return job
}

func (r *OrderMailCheckRunner) finishJob(jobId string, result OrderMailCheckResult) {
	r.mu.Lock()
	defer r.mu.Unlock()
	job := r.jobs[jobId]
	if job == nil {
		return
	}
	job.AffectedCount = result.AffectedCount
	job.FinishedTime = common.GetTimestamp()
	if result.Error != nil && !errors.Is(result.Error, context.Canceled) {
		job.Status = "failed"
		job.ErrorMessage = result.Error.Error()
		return
	}
	job.Status = "finished"
}
```

If `common.GetTimestampNano()` does not exist, use `time.Now().UnixNano()` and add a `time` import.

- [ ] **Step 7: Run job tests and commit**

Run:

```bash
cd /Users/ethan/Documents/yunbay
go test ./service -run 'TestRunSingleMailCheck|TestRunBatchMailCheck|TestVerifyLdxpMail|TestParseLdxpOrderMail' -count=1
```

Expected: PASS.

Commit:

```bash
git add service/order_mail_source.go service/order_mail_check_job.go service/order_mail_check_job_test.go model/order_management.go
git commit -m "feat: add order mail check runner"
```

---

## Task 5: Add DTOs and admin API controllers

**Files:**
- Create: `/Users/ethan/Documents/yunbay/dto/order_management.go`
- Create: `/Users/ethan/Documents/yunbay/controller/order_management.go`
- Create: `/Users/ethan/Documents/yunbay/controller/order_management_test.go`
- Modify: `/Users/ethan/Documents/yunbay/controller/audit.go`
- Modify: `/Users/ethan/Documents/yunbay/router/api-router.go`

- [ ] **Step 1: Write controller range parsing tests**

Create `/Users/ethan/Documents/yunbay/controller/order_management_test.go`:

```go
package controller

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestParseOrderManagementRange7dAnd30d(t *testing.T) {
	start, end, err := parseOrderManagementRange("7d", "", "", 1782887783)
	assert.NoError(t, err)
	assert.Equal(t, int64(1782282983), start)
	assert.Equal(t, int64(1782887783), end)

	start, end, err = parseOrderManagementRange("30d", "", "", 1782887783)
	assert.NoError(t, err)
	assert.Equal(t, int64(1780295783), start)
	assert.Equal(t, int64(1782887783), end)
}

func TestParseOrderManagementRangeCustom(t *testing.T) {
	start, end, err := parseOrderManagementRange("", "1782518400", "1782604800", 1782887783)
	assert.NoError(t, err)
	assert.Equal(t, int64(1782518400), start)
	assert.Equal(t, int64(1782604800), end)
}

func TestAffiliateWithdrawalActionRejectsEmptyRemarkOnReject(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/reject", func(c *gin.Context) {
		adminAffiliateWithdrawalReject(c)
	})
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/reject", nil)
	r.ServeHTTP(w, req)
	body := decodeTestResponse(t, w)
	assert.Equal(t, false, body["success"])
}
```

- [ ] **Step 2: Run controller tests and confirm expected failure**

Run:

```bash
cd /Users/ethan/Documents/yunbay
go test ./controller -run 'TestParseOrderManagementRange|TestAffiliateWithdrawalActionRejects' -count=1
```

Expected: FAIL because controller functions do not exist.

- [ ] **Step 3: Add DTO response shapes**

Create `/Users/ethan/Documents/yunbay/dto/order_management.go`:

```go
package dto

type OrderManagementMoneySummary struct {
	SiteAmount          float64 `json:"site_amount"`
	ExternalPaidAmount  float64 `json:"external_paid_amount"`
	OrderCount          int     `json:"order_count"`
	MailVerifiedCount   int     `json:"mail_verified_count"`
	MailPendingCount    int     `json:"mail_pending_count"`
	MailErrorCount      int     `json:"mail_error_count"`
	MailVerifiedRate    float64 `json:"mail_verified_rate"`
	AffiliateUserCount  int     `json:"affiliate_user_count"`
	AffiliateAmount     float64 `json:"affiliate_amount"`
	WithdrawalPendingCount  int     `json:"withdrawal_pending_count"`
	WithdrawalPendingAmount float64 `json:"withdrawal_pending_amount"`
}

type OrderManagementDailyPoint struct {
	Date               string  `json:"date"`
	SiteAmount         float64 `json:"site_amount"`
	ExternalPaidAmount float64 `json:"external_paid_amount"`
	OrderCount         int     `json:"order_count"`
	MailVerifiedCount  int     `json:"mail_verified_count"`
	MailErrorCount     int     `json:"mail_error_count"`
}

type OrderManagementAnalyticsResponse struct {
	Summary OrderManagementMoneySummary `json:"summary"`
	Daily   []OrderManagementDailyPoint `json:"daily"`
}

type OrderManagementAffiliateBrief struct {
	InviterUserId   int     `json:"inviter_user_id"`
	CommissionMoney float64 `json:"commission_money"`
	Status          string  `json:"status"`
}

type OrderManagementOrderItem struct {
	Id                 int                             `json:"id"`
	OrderType          string                          `json:"order_type"`
	SessionId          string                          `json:"session_id"`
	UserId             int                             `json:"user_id"`
	Username           string                          `json:"username"`
	SiteAmount         float64                         `json:"site_amount"`
	ExternalPaidAmount float64                         `json:"external_paid_amount"`
	WorkerOrderNo      string                          `json:"worker_order_no"`
	MailOrderNo        string                          `json:"mail_order_no"`
	MailPaidAmount     float64                         `json:"mail_paid_amount"`
	MailStatus         string                          `json:"mail_status"`
	MailStatusText     string                          `json:"mail_status_text"`
	ErrorCode          string                          `json:"error_code"`
	ErrorMessage       string                          `json:"error_message"`
	Affiliate          *OrderManagementAffiliateBrief  `json:"affiliate"`
	CreatedTime        int64                           `json:"created_time"`
	VerifiedTime       int64                           `json:"verified_time"`
}

type MailCheckRequest struct {
	Range string `json:"range"`
	Scope string `json:"scope"`
	Limit int    `json:"limit"`
}

type MailCheckResponse struct {
	JobId         string `json:"job_id"`
	Started       bool   `json:"started"`
	AffectedCount int    `json:"affected_count"`
}

type AffiliateWithdrawalDTO struct {
	Id            int     `json:"id"`
	WithdrawalId  string  `json:"withdrawal_id"`
	Amount        float64 `json:"amount"`
	Contact       string  `json:"contact"`
	Remark        string  `json:"remark"`
	Status        string  `json:"status"`
	CreatedTime   int64   `json:"created_time"`
	AdminRemark   string  `json:"admin_remark"`
	ProcessedTime int64   `json:"processed_time"`
}

type AffiliateStatsSummaryDTO struct {
	AffiliateUserCount                  int     `json:"affiliate_user_count"`
	PeriodCommissionAmount              float64 `json:"period_commission_amount"`
	PendingWithdrawalUserCount          int     `json:"pending_withdrawal_user_count"`
	PendingWithdrawalAmount             float64 `json:"pending_withdrawal_amount"`
	AvailableWithoutWithdrawalUserCount int     `json:"available_without_withdrawal_user_count"`
}

type AffiliateStatsItemDTO struct {
	UserId                 int                     `json:"user_id"`
	Username               string                  `json:"username"`
	PeriodCommissionAmount float64                 `json:"period_commission_amount"`
	TotalCommissionAmount  float64                 `json:"total_commission_amount"`
	AvailableAmount        float64                 `json:"available_amount"`
	WithdrawnAmount        float64                 `json:"withdrawn_amount"`
	Withdrawal             *AffiliateWithdrawalDTO `json:"withdrawal"`
}

type AffiliateStatsResponse struct {
	Summary AffiliateStatsSummaryDTO `json:"summary"`
	Items   []AffiliateStatsItemDTO  `json:"items"`
	Total   int64                    `json:"total"`
}

type WithdrawalActionRequest struct {
	AdminRemark string `json:"admin_remark"`
}
```

- [ ] **Step 4: Add controller helpers and handlers**

Create `/Users/ethan/Documents/yunbay/controller/order_management.go`. Include:

```go
package controller

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

func centsToAmount(cents int64) float64 {
	return float64(cents) / 100
}

func parseOrderManagementRange(rangeValue, startValue, endValue string, now int64) (int64, int64, error) {
	switch rangeValue {
	case "", "7d":
		return now - 7*24*60*60, now, nil
	case "30d":
		return now - 30*24*60*60, now, nil
	case "custom":
		// fall through to explicit timestamps
	default:
		return 0, 0, fmt.Errorf("invalid range: %s", rangeValue)
	}
	start, err := strconv.ParseInt(startValue, 10, 64)
	if err != nil || start <= 0 {
		return 0, 0, errors.New("invalid start_time")
	}
	end, err := strconv.ParseInt(endValue, 10, 64)
	if err != nil || end < start {
		return 0, 0, errors.New("invalid end_time")
	}
	return start, end, nil
}

func mailStatusText(status string) string {
	switch status {
	case model.MailCheckStatusNotRequired:
		return "不适用"
	case model.MailCheckStatusPending:
		return "待核对"
	case model.MailCheckStatusWaitingMail:
		return "待邮件"
	case model.MailCheckStatusChecking:
		return "核对中"
	case model.MailCheckStatusVerified:
		return "已核对"
	case model.MailCheckStatusOrderMismatch:
		return "单号异常"
	case model.MailCheckStatusAmountMismatch:
		return "金额异常"
	case model.MailCheckStatusMailParseFailed:
		return "邮件解析失败"
	case model.MailCheckStatusMailFetchFailed:
		return "邮件拉取失败"
	case model.MailCheckStatusTimeout:
		return "核对超时"
	default:
		return status
	}
}
```

Add handlers:

```go
func AdminOrderManagementAnalytics(c *gin.Context) {
	start, end, err := parseOrderManagementRange(c.Query("range"), c.Query("start_time"), c.Query("end_time"), common.GetTimestamp())
	if err != nil {
		common.ApiError(c, err)
		return
	}
	result, err := model.GetOrderManagementAnalytics(start, end)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	items := make([]dto.OrderManagementDailyPoint, 0, len(result.Daily))
	for _, point := range result.Daily {
		items = append(items, dto.OrderManagementDailyPoint{Date: point.Date, SiteAmount: centsToAmount(point.SiteAmountCents), ExternalPaidAmount: centsToAmount(point.ExternalPaidCents), OrderCount: point.OrderCount, MailVerifiedCount: point.MailVerifiedCount, MailErrorCount: point.MailErrorCount})
	}
	common.ApiSuccess(c, dto.OrderManagementAnalyticsResponse{Summary: dto.OrderManagementMoneySummary{SiteAmount: centsToAmount(result.Summary.SiteAmountCents), ExternalPaidAmount: centsToAmount(result.Summary.ExternalPaidCents), OrderCount: result.Summary.OrderCount, MailVerifiedCount: result.Summary.MailVerifiedCount, MailPendingCount: result.Summary.MailPendingCount, MailErrorCount: result.Summary.MailErrorCount, MailVerifiedRate: result.Summary.MailVerifiedRate, AffiliateUserCount: result.Summary.AffiliateUserCount, AffiliateAmount: centsToAmount(result.Summary.AffiliateAmountCents), WithdrawalPendingCount: result.Summary.PendingWithdrawalCount, WithdrawalPendingAmount: centsToAmount(result.Summary.PendingWithdrawalCents)}, Daily: items})
}

func AdminOrderManagementMailCheck(c *gin.Context) {
	var req dto.MailCheckRequest
	if c.Request.ContentLength > 0 {
		if err := c.ShouldBindJSON(&req); err != nil {
			common.ApiErrorMsg(c, "参数错误")
			return
		}
	}
	start, end, err := parseOrderManagementRange(req.Range, c.Query("start_time"), c.Query("end_time"), common.GetTimestamp())
	if err != nil {
		common.ApiError(c, err)
		return
	}
	job := service.DefaultOrderMailCheckRunner().StartBatch(c.Request.Context(), model.OrderMailCheckBatchFilter{StartTime: start, EndTime: end, Limit: req.Limit})
	recordManageAudit(c, "order.mail_check_batch", map[string]interface{}{"job_id": job.JobId, "range": req.Range, "limit": req.Limit})
	common.ApiSuccess(c, dto.MailCheckResponse{JobId: job.JobId, Started: true, AffectedCount: job.AffectedCount})
}

func AdminOrderManagementOrderMailCheck(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		common.ApiErrorMsg(c, "无效订单 ID")
		return
	}
	job := service.DefaultOrderMailCheckRunner().StartSingle(c.Request.Context(), id)
	recordManageAudit(c, "order.mail_check_single", map[string]interface{}{"job_id": job.JobId, "order_id": id})
	common.ApiSuccess(c, dto.MailCheckResponse{JobId: job.JobId, Started: true, AffectedCount: job.AffectedCount})
}

func AdminOrderManagementMailCheckJob(c *gin.Context) {
	job, ok := service.DefaultOrderMailCheckRunner().GetJob(c.Param("job_id"))
	if !ok {
		common.ApiErrorMsg(c, "核对任务不存在")
		return
	}
	common.ApiSuccess(c, job)
}
```

Also add `AdminOrderManagementOrders`, `AdminOrderManagementAffiliateStats`, `AdminAffiliateWithdrawalPaid`, `AdminAffiliateWithdrawalReject`, and `AdminAffiliateSourceOrders`. Keep each handler small and map model cents to floats with `centsToAmount`.

For rejection, require a non-empty `admin_remark`:

```go
func adminAffiliateWithdrawalReject(c *gin.Context) {
	var req dto.WithdrawalActionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiErrorMsg(c, "参数错误")
		return
	}
	if strings.TrimSpace(req.AdminRemark) == "" {
		common.ApiErrorMsg(c, "驳回提现必须填写管理员备注")
		return
	}
	id, _ := strconv.Atoi(c.Param("id"))
	withdrawal, err := model.RejectAffiliateWithdrawal(id, c.GetInt("id"), req.AdminRemark)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	recordManageAudit(c, "affiliate.withdrawal_reject", map[string]interface{}{"withdrawal_id": withdrawal.WithdrawalId, "user_id": withdrawal.UserId, "amount": centsToAmount(withdrawal.AmountCents)})
	common.ApiSuccess(c, withdrawal)
}
```

- [ ] **Step 5: Add audit templates**

In `/Users/ethan/Documents/yunbay/controller/audit.go`, add to `auditContentTemplates`:

```go
	"order.mail_check_single":      "Started mail verification for order ${order_id} (${job_id})",
	"order.mail_check_batch":       "Started batch mail verification (${job_id}, range ${range}, limit ${limit})",
	"affiliate.withdrawal_paid":    "Marked affiliate withdrawal ${withdrawal_id} as paid for user ${user_id} (${amount})",
	"affiliate.withdrawal_reject":  "Rejected affiliate withdrawal ${withdrawal_id} for user ${user_id} (${amount})",
```

- [ ] **Step 6: Register routes**

In `/Users/ethan/Documents/yunbay/router/api-router.go`, after the subscription admin route block, add:

```go
		orderManagementAdminRoute := apiRouter.Group("/order-management/admin")
		orderManagementAdminRoute.Use(middleware.AdminAuth())
		{
			orderManagementAdminRoute.GET("/analytics", controller.AdminOrderManagementAnalytics)
			orderManagementAdminRoute.GET("/orders", controller.AdminOrderManagementOrders)
			orderManagementAdminRoute.POST("/orders/:id/mail-check", controller.AdminOrderManagementOrderMailCheck)
			orderManagementAdminRoute.POST("/mail-check", controller.AdminOrderManagementMailCheck)
			orderManagementAdminRoute.GET("/mail-check/:job_id", controller.AdminOrderManagementMailCheckJob)
			orderManagementAdminRoute.GET("/affiliate-stats", controller.AdminOrderManagementAffiliateStats)
			orderManagementAdminRoute.GET("/affiliate-stats/:user_id/source-orders", controller.AdminAffiliateSourceOrders)
			orderManagementAdminRoute.POST("/affiliate-withdrawals/:id/paid", controller.AdminAffiliateWithdrawalPaid)
			orderManagementAdminRoute.POST("/affiliate-withdrawals/:id/reject", controller.AdminAffiliateWithdrawalReject)
		}
```

- [ ] **Step 7: Run controller and backend package tests, then commit**

Run:

```bash
cd /Users/ethan/Documents/yunbay
go test ./controller -run 'TestParseOrderManagementRange|TestAffiliateWithdrawalActionRejects' -count=1
go test ./model ./service ./controller -run 'TestOrderManagement|TestAffiliateWithdrawal|TestParseLdxp|TestVerifyLdxp|TestRunSingleMailCheck|TestRunBatchMailCheck|TestParseOrderManagementRange' -count=1
```

Expected: PASS.

Commit:

```bash
git add dto/order_management.go controller/order_management.go controller/order_management_test.go controller/audit.go router/api-router.go
git commit -m "feat: expose admin order management APIs"
```

---

## Task 6: Add frontend route, API client, types, and formatting helpers

**Files:**
- Create: `/Users/ethan/Documents/yunbay/web/default/src/routes/_authenticated/order-management/index.tsx`
- Create: `/Users/ethan/Documents/yunbay/web/default/src/features/order-management/api.ts`
- Create: `/Users/ethan/Documents/yunbay/web/default/src/features/order-management/types.ts`
- Create: `/Users/ethan/Documents/yunbay/web/default/src/features/order-management/lib/format.ts`
- Create: `/Users/ethan/Documents/yunbay/web/default/src/features/order-management/lib/format.test.ts`

- [ ] **Step 1: Write formatting helper tests**

Create `/Users/ethan/Documents/yunbay/web/default/src/features/order-management/lib/format.test.ts`:

```ts
import assert from 'node:assert/strict'
import test from 'node:test'
import {
  formatCny,
  formatPercentRate,
  getMailStatusLabelKey,
} from './format'

test('formatCny renders CNY with two decimals', () => {
  assert.equal(formatCny(10), '¥10.00')
  assert.equal(formatCny(10.3), '¥10.30')
  assert.equal(formatCny(425), '¥425.00')
})

test('formatPercentRate renders one decimal place', () => {
  assert.equal(formatPercentRate(0.968), '96.8%')
  assert.equal(formatPercentRate(1), '100.0%')
})

test('mail status labels are stable i18n keys', () => {
  assert.equal(getMailStatusLabelKey('verified'), 'Verified')
  assert.equal(getMailStatusLabelKey('amount_mismatch'), 'Amount mismatch')
  assert.equal(getMailStatusLabelKey('order_mismatch'), 'Order number mismatch')
  assert.equal(getMailStatusLabelKey('waiting_mail'), 'Pending mail')
})
```

- [ ] **Step 2: Run formatting tests and confirm expected failure**

Run:

```bash
cd /Users/ethan/Documents/yunbay/web/default
bun test src/features/order-management/lib/format.test.ts
```

Expected: FAIL because the helper file does not exist.

- [ ] **Step 3: Add TypeScript API types**

Create `/Users/ethan/Documents/yunbay/web/default/src/features/order-management/types.ts`:

```ts
export type MailCheckStatus =
  | 'not_required'
  | 'pending'
  | 'waiting_mail'
  | 'checking'
  | 'verified'
  | 'order_mismatch'
  | 'amount_mismatch'
  | 'mail_parse_failed'
  | 'mail_fetch_failed'
  | 'timeout'

export type DateRangeKey = '7d' | '30d' | 'custom'

export interface ApiResponse<T = unknown> {
  success: boolean
  message?: string
  data?: T
}

export interface OrderManagementSummary {
  site_amount: number
  external_paid_amount: number
  order_count: number
  mail_verified_count: number
  mail_pending_count: number
  mail_error_count: number
  mail_verified_rate: number
  affiliate_user_count: number
  affiliate_amount: number
  withdrawal_pending_count: number
  withdrawal_pending_amount: number
}

export interface OrderDailyPoint {
  date: string
  site_amount: number
  external_paid_amount: number
  order_count: number
  mail_verified_count: number
  mail_error_count: number
}

export interface OrderAnalyticsResponse {
  summary: OrderManagementSummary
  daily: OrderDailyPoint[]
}

export interface OrderAffiliateBrief {
  inviter_user_id: number
  commission_money: number
  status: string
}

export interface OrderManagementOrderItem {
  id: number
  order_type: string
  session_id: string
  user_id: number
  username: string
  site_amount: number
  external_paid_amount: number
  worker_order_no: string
  mail_order_no: string
  mail_paid_amount: number
  mail_status: MailCheckStatus
  mail_status_text: string
  error_code: string
  error_message: string
  affiliate?: OrderAffiliateBrief | null
  created_time: number
  verified_time: number
}

export interface PageData<T> {
  page: number
  page_size: number
  total: number
  items: T[]
}

export interface MailCheckResponse {
  job_id: string
  started: boolean
  affected_count: number
}

export interface MailCheckJobStatus {
  job_id: string
  status: 'running' | 'finished' | 'failed'
  affected_count: number
  error_message: string
  created_time: number
  finished_time: number
}

export interface AffiliateWithdrawal {
  id: number
  withdrawal_id: string
  amount: number
  contact: string
  remark: string
  status: 'pending' | 'paid' | 'rejected'
  created_time: number
  admin_remark: string
  processed_time: number
}

export interface AffiliateStatsSummary {
  affiliate_user_count: number
  period_commission_amount: number
  pending_withdrawal_user_count: number
  pending_withdrawal_amount: number
  available_without_withdrawal_user_count: number
}

export interface AffiliateStatsItem {
  user_id: number
  username: string
  period_commission_amount: number
  total_commission_amount: number
  available_amount: number
  withdrawn_amount: number
  withdrawal?: AffiliateWithdrawal | null
}

export interface AffiliateStatsResponse {
  summary: AffiliateStatsSummary
  items: AffiliateStatsItem[]
  total: number
}
```

- [ ] **Step 4: Add formatting helpers**

Create `/Users/ethan/Documents/yunbay/web/default/src/features/order-management/lib/format.ts`:

```ts
import type { MailCheckStatus } from '../types'

export function formatCny(value: number): string {
  if (!Number.isFinite(value)) return '¥0.00'
  return `¥${value.toFixed(2)}`
}

export function formatPercentRate(value: number): string {
  if (!Number.isFinite(value)) return '0.0%'
  return `${(value * 100).toFixed(1)}%`
}

export function formatUnixTime(value?: number): string {
  if (!value) return '-'
  return new Date(value * 1000).toLocaleString()
}

export function getMailStatusLabelKey(status: MailCheckStatus | string): string {
  const labels: Record<string, string> = {
    not_required: 'Not required',
    pending: 'Pending verification',
    waiting_mail: 'Pending mail',
    checking: 'Checking...',
    verified: 'Verified',
    order_mismatch: 'Order number mismatch',
    amount_mismatch: 'Amount mismatch',
    mail_parse_failed: 'Mail parse failed',
    mail_fetch_failed: 'Mail fetch failed',
    timeout: 'Verification timeout',
  }
  return labels[status] ?? status
}

export function isMailStatusError(status: MailCheckStatus | string): boolean {
  return [
    'order_mismatch',
    'amount_mismatch',
    'mail_parse_failed',
    'mail_fetch_failed',
    'timeout',
  ].includes(status)
}
```

- [ ] **Step 5: Add frontend API client**

Create `/Users/ethan/Documents/yunbay/web/default/src/features/order-management/api.ts`:

```ts
import { api } from '@/lib/api'
import type {
  AffiliateStatsResponse,
  ApiResponse,
  MailCheckJobStatus,
  MailCheckResponse,
  OrderAnalyticsResponse,
  OrderManagementOrderItem,
  PageData,
} from './types'

function withDefinedParams(params: Record<string, unknown>) {
  const query = new URLSearchParams()
  Object.entries(params).forEach(([key, value]) => {
    if (value !== undefined && value !== null && value !== '') {
      query.set(key, String(value))
    }
  })
  return query.toString()
}

export async function getOrderAnalytics(params: Record<string, unknown>) {
  const res = await api.get<ApiResponse<OrderAnalyticsResponse>>(
    `/api/order-management/admin/analytics?${withDefinedParams(params)}`
  )
  return res.data
}

export async function getOrderManagementOrders(params: Record<string, unknown>) {
  const res = await api.get<ApiResponse<PageData<OrderManagementOrderItem>>>(
    `/api/order-management/admin/orders?${withDefinedParams(params)}`
  )
  return res.data
}

export async function startSingleMailCheck(orderId: number) {
  const res = await api.post<ApiResponse<MailCheckResponse>>(
    `/api/order-management/admin/orders/${orderId}/mail-check`
  )
  return res.data
}

export async function startBatchMailCheck(payload: Record<string, unknown>) {
  const res = await api.post<ApiResponse<MailCheckResponse>>(
    '/api/order-management/admin/mail-check',
    payload
  )
  return res.data
}

export async function getMailCheckJob(jobId: string) {
  const res = await api.get<ApiResponse<MailCheckJobStatus>>(
    `/api/order-management/admin/mail-check/${jobId}`,
    { disableDuplicate: true }
  )
  return res.data
}

export async function getAffiliateStats(params: Record<string, unknown>) {
  const res = await api.get<ApiResponse<AffiliateStatsResponse>>(
    `/api/order-management/admin/affiliate-stats?${withDefinedParams(params)}`
  )
  return res.data
}

export async function markWithdrawalPaid(id: number, adminRemark: string) {
  const res = await api.post<ApiResponse>(
    `/api/order-management/admin/affiliate-withdrawals/${id}/paid`,
    { admin_remark: adminRemark }
  )
  return res.data
}

export async function rejectWithdrawal(id: number, adminRemark: string) {
  const res = await api.post<ApiResponse>(
    `/api/order-management/admin/affiliate-withdrawals/${id}/reject`,
    { admin_remark: adminRemark }
  )
  return res.data
}
```

- [ ] **Step 6: Add admin-only route**

Create `/Users/ethan/Documents/yunbay/web/default/src/routes/_authenticated/order-management/index.tsx`:

```tsx
import z from 'zod'
import { createFileRoute, redirect } from '@tanstack/react-router'
import { ROLE } from '@/lib/roles'
import { useAuthStore } from '@/stores/auth-store'
import { OrderManagement } from '@/features/order-management'

const orderManagementSearchSchema = z.object({
  range: z.enum(['7d', '30d', 'custom']).optional().catch('7d'),
  start_time: z.number().optional().catch(undefined),
  end_time: z.number().optional().catch(undefined),
  page: z.number().optional().catch(1),
  page_size: z.number().optional().catch(20),
  mail_status: z.string().optional().catch(''),
  keyword: z.string().optional().catch(''),
})

export const Route = createFileRoute('/_authenticated/order-management/')({
  beforeLoad: () => {
    const { auth } = useAuthStore.getState()
    if (!auth.user || auth.user.role < ROLE.ADMIN) {
      throw redirect({ to: '/403' })
    }
  },
  validateSearch: orderManagementSearchSchema,
  component: OrderManagement,
})
```

- [ ] **Step 7: Run frontend helper tests and commit**

Run:

```bash
cd /Users/ethan/Documents/yunbay/web/default
bun test src/features/order-management/lib/format.test.ts
```

Expected: PASS.

Commit:

```bash
cd /Users/ethan/Documents/yunbay
git add web/default/src/routes/_authenticated/order-management/index.tsx web/default/src/features/order-management/api.ts web/default/src/features/order-management/types.ts web/default/src/features/order-management/lib/format.ts web/default/src/features/order-management/lib/format.test.ts
git commit -m "feat: add order management frontend API shell"
```

---

## Task 7: Build the frontend analytics and order detail page

**Files:**
- Create: `/Users/ethan/Documents/yunbay/web/default/src/features/order-management/index.tsx`
- Create: `/Users/ethan/Documents/yunbay/web/default/src/features/order-management/components/range-toolbar.tsx`
- Create: `/Users/ethan/Documents/yunbay/web/default/src/features/order-management/components/order-analytics-cards.tsx`
- Create: `/Users/ethan/Documents/yunbay/web/default/src/features/order-management/components/order-trend-chart.tsx`
- Create: `/Users/ethan/Documents/yunbay/web/default/src/features/order-management/components/mail-check-status-badge.tsx`
- Create: `/Users/ethan/Documents/yunbay/web/default/src/features/order-management/components/order-details-table.tsx`

- [ ] **Step 1: Add the status badge component**

Create `/Users/ethan/Documents/yunbay/web/default/src/features/order-management/components/mail-check-status-badge.tsx`:

```tsx
import { useTranslation } from 'react-i18next'
import { Badge } from '@/components/ui/badge'
import { getMailStatusLabelKey, isMailStatusError } from '../lib/format'
import type { MailCheckStatus } from '../types'

export function MailCheckStatusBadge({ status }: { status: MailCheckStatus | string }) {
  const { t } = useTranslation()
  if (status === 'verified') {
    return <Badge variant='default'>{t(getMailStatusLabelKey(status))}</Badge>
  }
  if (status === 'checking') {
    return <Badge variant='secondary'>{t(getMailStatusLabelKey(status))}</Badge>
  }
  if (isMailStatusError(status)) {
    return <Badge variant='destructive'>{t(getMailStatusLabelKey(status))}</Badge>
  }
  return <Badge variant='outline'>{t(getMailStatusLabelKey(status))}</Badge>
}
```

- [ ] **Step 2: Add range toolbar**

Create `/Users/ethan/Documents/yunbay/web/default/src/features/order-management/components/range-toolbar.tsx` with `ToggleGroup` for `7d` and `30d`, date inputs for custom timestamps, and two buttons: `Fetch latest mail` and `Verify unfinished orders now`. Use `Button`, `Input`, and `ToggleGroup` from existing `@/components/ui` files. Icons are optional; if used, import from the configured icon library or existing project usage.

Required props:

```ts
interface RangeToolbarProps {
  range: '7d' | '30d' | 'custom'
  startTime?: number
  endTime?: number
  isChecking: boolean
  onRangeChange: (range: '7d' | '30d' | 'custom') => void
  onCustomRangeChange: (startTime?: number, endTime?: number) => void
  onBatchCheck: () => void
  onExportCsv: () => void
}
```

- [ ] **Step 3: Add KPI cards**

Create `/Users/ethan/Documents/yunbay/web/default/src/features/order-management/components/order-analytics-cards.tsx` using `Card`, `CardHeader`, `CardTitle`, `CardDescription`, and `CardContent`. Render five cards: current range revenue, 30-day entry/summary label, external paid amount, mail verification rate, pending verification count.

Use:

```tsx
formatCny(summary.site_amount)
formatCny(summary.external_paid_amount)
formatPercentRate(summary.mail_verified_rate)
String(summary.mail_pending_count + summary.mail_error_count)
```

- [ ] **Step 4: Add VChart trend chart**

Create `/Users/ethan/Documents/yunbay/web/default/src/features/order-management/components/order-trend-chart.tsx` with `VChart` from `@visactor/react-vchart`. Use `Card` composition and a spec with daily points mapped to two bar series:

```ts
const data = daily.flatMap((point) => [
  { date: point.date, type: t('Revenue amount'), value: point.site_amount },
  { date: point.date, type: t('External paid amount'), value: point.external_paid_amount },
])
```

Use a grouped bar chart spec:

```ts
{
  type: 'bar',
  data: [{ id: 'revenue', values: data }],
  xField: ['date', 'type'],
  yField: 'value',
  seriesField: 'type',
  legends: { visible: true, orient: 'bottom' },
  axes: [
    { orient: 'bottom', type: 'band' },
    { orient: 'left', type: 'linear' },
  ],
  tooltip: { visible: true }
}
```

- [ ] **Step 5: Add large order details table**

Create `/Users/ethan/Documents/yunbay/web/default/src/features/order-management/components/order-details-table.tsx` using `Card`, `Table`, `ScrollArea`, `Button`, and `MailCheckStatusBadge`. Columns must include: time, user, local order/session, site amount, external paid, mail paid, worker order number, mail verification, affiliate, actions.

Button behavior:

```tsx
<Button
  variant='outline'
  size='sm'
  disabled={row.mail_status === 'checking' || verifyingId === row.id}
  onClick={() => onVerify(row.id)}
>
  {row.mail_status === 'verified' ? t('Recheck') : t('Verify now')}
</Button>
```

Use red-tinted row classes only for error statuses with semantic muted/destructive classes:

```tsx
className={isMailStatusError(row.mail_status) ? 'bg-destructive/5' : undefined}
```

- [ ] **Step 6: Add page shell and queries**

Create `/Users/ethan/Documents/yunbay/web/default/src/features/order-management/index.tsx`:

- Use `getRouteApi('/_authenticated/order-management/')` to read search params and navigate.
- Use React Query for analytics, orders, and mail check mutation.
- Layout order:
  1. `SectionPageLayout` title/actions.
  2. `RangeToolbar`.
  3. `OrderAnalyticsCards`.
  4. `OrderTrendChart`.
  5. `OrderDetailsTable` in a larger card.
  6. Affiliate stats section from Task 8.
- After `startSingleMailCheck` or `startBatchMailCheck`, invalidate analytics, orders, and affiliate stats queries.

Query keys:

```ts
const orderManagementKeys = {
  analytics: (params: Record<string, unknown>) => ['order-management', 'analytics', params] as const,
  orders: (params: Record<string, unknown>) => ['order-management', 'orders', params] as const,
  affiliate: (params: Record<string, unknown>) => ['order-management', 'affiliate', params] as const,
}
```

- [ ] **Step 7: Run typecheck for the new route and fix route tree generation**

Run:

```bash
cd /Users/ethan/Documents/yunbay/web/default
bun run typecheck
```

Expected: If the generated TanStack route tree is stale, the command reports route type errors. Run the existing build/dev route generation path by executing:

```bash
bun run build
```

Then re-run:

```bash
bun run typecheck
```

Expected: PASS after route tree generation and type fixes.

- [ ] **Step 8: Commit frontend page core**

```bash
cd /Users/ethan/Documents/yunbay
git add web/default/src/features/order-management/index.tsx web/default/src/features/order-management/components/range-toolbar.tsx web/default/src/features/order-management/components/order-analytics-cards.tsx web/default/src/features/order-management/components/order-trend-chart.tsx web/default/src/features/order-management/components/mail-check-status-badge.tsx web/default/src/features/order-management/components/order-details-table.tsx web/default/src/routeTree.gen.ts
git commit -m "feat: build admin order management page"
```

---

## Task 8: Build affiliate statistics, withdrawal actions, and source orders UI

**Files:**
- Create: `/Users/ethan/Documents/yunbay/web/default/src/features/order-management/components/affiliate-stats-section.tsx`
- Create: `/Users/ethan/Documents/yunbay/web/default/src/features/order-management/components/withdrawal-actions.tsx`
- Create: `/Users/ethan/Documents/yunbay/web/default/src/features/order-management/components/source-orders-drawer.tsx`
- Modify: `/Users/ethan/Documents/yunbay/web/default/src/features/order-management/api.ts`
- Modify: `/Users/ethan/Documents/yunbay/web/default/src/features/order-management/types.ts`
- Modify: `/Users/ethan/Documents/yunbay/web/default/src/features/order-management/index.tsx`

- [ ] **Step 1: Add source order types and API client**

Extend `/Users/ethan/Documents/yunbay/web/default/src/features/order-management/types.ts`:

```ts
export interface AffiliateSourceOrder {
  order_time: number
  invitee_user_id: number
  invitee_username: string
  trade_no: string
  worker_order_no: string
  base_money: number
  rate_bps: number
  commission_money: number
  mail_status: MailCheckStatus
}
```

Extend `/Users/ethan/Documents/yunbay/web/default/src/features/order-management/api.ts`:

```ts
import type { AffiliateSourceOrder } from './types'

export async function getAffiliateSourceOrders(userId: number, params: Record<string, unknown>) {
  const res = await api.get<ApiResponse<AffiliateSourceOrder[]>>(
    `/api/order-management/admin/affiliate-stats/${userId}/source-orders?${withDefinedParams(params)}`
  )
  return res.data
}
```

- [ ] **Step 2: Add withdrawal action dialog component**

Create `/Users/ethan/Documents/yunbay/web/default/src/features/order-management/components/withdrawal-actions.tsx` using existing `Dialog`, `Button`, `Textarea`, and `toast` patterns. Props:

```ts
interface WithdrawalActionsProps {
  withdrawalId: number
  status: string
  onPaid: (remark: string) => Promise<void>
  onReject: (remark: string) => Promise<void>
}
```

Behavior:

- Hide action buttons when status is not `pending`.
- `Mark as paid` allows an optional remark.
- `Reject withdrawal` requires a non-empty remark and shows `Please enter an admin remark` when empty.
- Disable buttons while mutation is running.

- [ ] **Step 3: Add source orders drawer**

Create `/Users/ethan/Documents/yunbay/web/default/src/features/order-management/components/source-orders-drawer.tsx` using `Sheet` or `Drawer` with an accessible title. It accepts `userId`, `open`, `onOpenChange`, `range`, `startTime`, and `endTime`. Fetch source orders with `getAffiliateSourceOrders`. Render columns: order time, invitee user, trade/order number, base money, rate, commission, mail status.

Use:

```tsx
{(order.rate_bps / 100).toFixed(2)}%
formatCny(order.base_money)
formatCny(order.commission_money)
<MailCheckStatusBadge status={order.mail_status} />
```

- [ ] **Step 4: Add affiliate stats section**

Create `/Users/ethan/Documents/yunbay/web/default/src/features/order-management/components/affiliate-stats-section.tsx` with:

- KPI cards for users with rewards, period commission, pending withdrawal users/amount, available without withdrawal.
- Table rows for each `AffiliateStatsItem`.
- Inline display of withdrawal contact, amount, created time, remark, admin remark, status.
- `WithdrawalActions` for pending withdrawals.
- `Source orders` button opens `SourceOrdersDrawer`.

- [ ] **Step 5: Wire affiliate section into the page**

In `/Users/ethan/Documents/yunbay/web/default/src/features/order-management/index.tsx`, pass the current range and custom timestamps into `AffiliateStatsSection`, and invalidate the affiliate query after paid/reject mutations.

- [ ] **Step 6: Run frontend typecheck and commit**

Run:

```bash
cd /Users/ethan/Documents/yunbay/web/default
bun run typecheck
```

Expected: PASS.

Commit:

```bash
cd /Users/ethan/Documents/yunbay
git add web/default/src/features/order-management/components/affiliate-stats-section.tsx web/default/src/features/order-management/components/withdrawal-actions.tsx web/default/src/features/order-management/components/source-orders-drawer.tsx web/default/src/features/order-management/api.ts web/default/src/features/order-management/types.ts web/default/src/features/order-management/index.tsx
git commit -m "feat: add affiliate stats and withdrawal management UI"
```

---

## Task 9: Add sidebar navigation and generated sidebar defaults

**Files:**
- Modify: `/Users/ethan/Documents/yunbay/web/default/src/hooks/sidebar-data-model.ts`
- Modify: `/Users/ethan/Documents/yunbay/web/default/src/hooks/sidebar-data-model.test.ts`
- Modify: `/Users/ethan/Documents/yunbay/web/default/src/hooks/use-sidebar-config.ts`
- Modify: `/Users/ethan/Documents/yunbay/web/default/src/hooks/use-sidebar-data.ts`
- Modify: `/Users/ethan/Documents/yunbay/model/user.go`
- Modify: `/Users/ethan/Documents/yunbay/controller/user.go`

- [ ] **Step 1: Update sidebar model tests first**

In `/Users/ethan/Documents/yunbay/web/default/src/hooks/sidebar-data-model.test.ts`, add to the admin test:

```ts
  assert.equal(
    items.some((item) => 'url' in item && item.url === '/order-management'),
    true
  )
```

Add a new ordinary-user assertion:

```ts
  assert.equal(
    items.some((item) => 'url' in item && item.url === '/order-management'),
    false
  )
```

- [ ] **Step 2: Run sidebar tests and confirm expected failure**

Run:

```bash
cd /Users/ethan/Documents/yunbay/web/default
bun test src/hooks/sidebar-data-model.test.ts
```

Expected: FAIL because the admin nav entry does not exist.

- [ ] **Step 3: Add an order-management icon slot and nav item**

In `/Users/ethan/Documents/yunbay/web/default/src/hooks/sidebar-data-model.ts`:

- Add `receipt` or `creditCard` to `SidebarIconMap` if not already available.
- Add this item to the admin group after `Users` or before `Redemption Codes`:

```ts
          {
            title: t('Order Management'),
            url: '/order-management',
            icon: icons.creditCard,
          },
```

- [ ] **Step 4: Add URL mapping and default module config**

In `/Users/ethan/Documents/yunbay/web/default/src/hooks/use-sidebar-config.ts`:

- Add `order_management: true` to `DEFAULT_SIDEBAR_MODULES.admin`.
- Add URL mapping:

```ts
  '/order-management': { section: 'admin', module: 'order_management' },
```

- [ ] **Step 5: Update frontend icon source map**

In `/Users/ethan/Documents/yunbay/web/default/src/hooks/use-sidebar-data.ts`, ensure the `creditCard` icon is present in the icon map passed to `buildSidebarData`. If it is already present, leave the file unchanged.

- [ ] **Step 6: Update backend-generated sidebar defaults for new users**

In both `/Users/ethan/Documents/yunbay/model/user.go` and `/Users/ethan/Documents/yunbay/controller/user.go`, add `"order_management": true,` to admin/root generated config maps:

```go
		defaultConfig["admin"] = map[string]interface{}{
			"enabled":          true,
			"channel":          true,
			"models":           true,
			"redemption":       true,
			"user":             true,
			"order_management": true,
			"setting":          false,
		}
```

For root config, keep `setting: true`.

- [ ] **Step 7: Run sidebar tests and commit**

Run:

```bash
cd /Users/ethan/Documents/yunbay/web/default
bun test src/hooks/sidebar-data-model.test.ts
```

Expected: PASS.

Commit:

```bash
cd /Users/ethan/Documents/yunbay
git add web/default/src/hooks/sidebar-data-model.ts web/default/src/hooks/sidebar-data-model.test.ts web/default/src/hooks/use-sidebar-config.ts web/default/src/hooks/use-sidebar-data.ts model/user.go controller/user.go
git commit -m "feat: add order management sidebar entry"
```

---

## Task 10: Add frontend i18n translations

**Files:**
- Modify: `/Users/ethan/Documents/yunbay/web/default/src/i18n/locales/en.json`
- Modify: `/Users/ethan/Documents/yunbay/web/default/src/i18n/locales/zh.json`
- Modify: `/Users/ethan/Documents/yunbay/web/default/src/i18n/locales/fr.json`
- Modify: `/Users/ethan/Documents/yunbay/web/default/src/i18n/locales/ja.json`
- Modify: `/Users/ethan/Documents/yunbay/web/default/src/i18n/locales/ru.json`
- Modify: `/Users/ethan/Documents/yunbay/web/default/src/i18n/locales/vi.json`

- [ ] **Step 1: Run i18n sync and read the report**

Run:

```bash
cd /Users/ethan/Documents/yunbay/web/default
bun run i18n:sync
cat src/i18n/locales/_reports/_sync-report.json
```

Expected: The report shows missing or untranslated order-management keys before translations are added.

- [ ] **Step 2: Add translation keys with a script**

Create `/Users/ethan/Documents/yunbay/web/default/scripts/add-order-management-i18n.mjs` with translations for these keys:

```js
const newKeys = {
  en: {
    'Order Management': 'Order Management',
    'Order analytics': 'Order analytics',
    'Revenue amount': 'Revenue amount',
    '30-day revenue': '30-day revenue',
    'External paid amount': 'External paid amount',
    'Mail verification': 'Mail verification',
    'Pending verification': 'Pending verification',
    'Pending mail': 'Pending mail',
    'Verify now': 'Verify now',
    'Verify unfinished orders now': 'Verify unfinished orders now',
    'Fetch latest mail': 'Fetch latest mail',
    'Recheck': 'Recheck',
    'Checking...': 'Checking...',
    'Verified': 'Verified',
    'Not required': 'Not required',
    'Amount mismatch': 'Amount mismatch',
    'Order number mismatch': 'Order number mismatch',
    'Mail parse failed': 'Mail parse failed',
    'Mail fetch failed': 'Mail fetch failed',
    'Verification timeout': 'Verification timeout',
    'Order details': 'Order details',
    'Site amount': 'Site amount',
    'Mail paid amount': 'Mail paid amount',
    'Worker order number': 'Worker order number',
    'Affiliate': 'Affiliate',
    'Affiliate statistics': 'Affiliate statistics',
    'Users with rewards': 'Users with rewards',
    'Period rewards': 'Period rewards',
    'Total rewards': 'Total rewards',
    'Available rewards': 'Available rewards',
    'Withdrawn rewards': 'Withdrawn rewards',
    'Withdrawal request': 'Withdrawal request',
    'Pending withdrawals': 'Pending withdrawals',
    'Available without withdrawal': 'Available without withdrawal',
    'Mark as paid': 'Mark as paid',
    'Reject withdrawal': 'Reject withdrawal',
    'Source orders': 'Source orders',
    'Admin remark': 'Admin remark',
    'Please enter an admin remark': 'Please enter an admin remark',
    'No order details found': 'No order details found',
    'No affiliate records found': 'No affiliate records found',
    'Export CSV': 'Export CSV',
    'Last 7 days': 'Last 7 days',
    'Last 30 days': 'Last 30 days',
    'Custom range': 'Custom range'
  },
  zh: {
    'Order Management': '订单管理',
    'Order analytics': '订单分析',
    'Revenue amount': '入账金额',
    '30-day revenue': '30 天入账',
    'External paid amount': '外部实付金额',
    'Mail verification': '邮件核对',
    'Pending verification': '待核对',
    'Pending mail': '待邮件',
    'Verify now': '立即核对',
    'Verify unfinished orders now': '立即核对未完成订单',
    'Fetch latest mail': '拉取最新邮件',
    'Recheck': '重新核对',
    'Checking...': '核对中...',
    'Verified': '已核对',
    'Not required': '不适用',
    'Amount mismatch': '金额异常',
    'Order number mismatch': '单号异常',
    'Mail parse failed': '邮件解析失败',
    'Mail fetch failed': '邮件拉取失败',
    'Verification timeout': '核对超时',
    'Order details': '成交明细',
    'Site amount': '入账金额',
    'Mail paid amount': '邮件实付',
    'Worker order number': '小铺单号',
    'Affiliate': '返利',
    'Affiliate statistics': '返利统计',
    'Users with rewards': '有返利用户',
    'Period rewards': '周期返利',
    'Total rewards': '累计返利',
    'Available rewards': '可提现',
    'Withdrawn rewards': '已提现',
    'Withdrawal request': '提现申请',
    'Pending withdrawals': '待处理提现',
    'Available without withdrawal': '可提现未申请',
    'Mark as paid': '标记已打款',
    'Reject withdrawal': '驳回提现',
    'Source orders': '来源订单',
    'Admin remark': '管理员备注',
    'Please enter an admin remark': '请填写管理员备注',
    'No order details found': '暂无成交明细',
    'No affiliate records found': '暂无返利记录',
    'Export CSV': '导出 CSV',
    'Last 7 days': '近 7 天',
    'Last 30 days': '近 30 天',
    'Custom range': '自定义时间'
  },
  fr: {
    'Order Management': 'Gestion des commandes',
    'Order analytics': 'Analyse des commandes',
    'Revenue amount': 'Montant crédité',
    '30-day revenue': 'Revenu sur 30 jours',
    'External paid amount': 'Montant payé externe',
    'Mail verification': 'Vérification e-mail',
    'Pending verification': 'Vérification en attente',
    'Pending mail': 'E-mail en attente',
    'Verify now': 'Vérifier maintenant',
    'Verify unfinished orders now': 'Vérifier les commandes incomplètes maintenant',
    'Fetch latest mail': 'Récupérer les derniers e-mails',
    'Recheck': 'Revérifier',
    'Checking...': 'Vérification...',
    'Verified': 'Vérifié',
    'Not required': 'Non requis',
    'Amount mismatch': 'Montant incohérent',
    'Order number mismatch': 'Numéro de commande incohérent',
    'Mail parse failed': "Échec de l'analyse de l'e-mail",
    'Mail fetch failed': "Échec de récupération de l'e-mail",
    'Verification timeout': 'Délai de vérification dépassé',
    'Order details': 'Détails des transactions',
    'Site amount': 'Montant crédité',
    'Mail paid amount': 'Montant payé dans l’e-mail',
    'Worker order number': 'Numéro de commande boutique',
    'Affiliate': 'Affiliation',
    'Affiliate statistics': 'Statistiques d’affiliation',
    'Users with rewards': 'Utilisateurs avec récompenses',
    'Period rewards': 'Récompenses de la période',
    'Total rewards': 'Récompenses totales',
    'Available rewards': 'Disponible au retrait',
    'Withdrawn rewards': 'Déjà retiré',
    'Withdrawal request': 'Demande de retrait',
    'Pending withdrawals': 'Retraits en attente',
    'Available without withdrawal': 'Disponible sans demande',
    'Mark as paid': 'Marquer comme payé',
    'Reject withdrawal': 'Rejeter le retrait',
    'Source orders': 'Commandes sources',
    'Admin remark': 'Remarque administrateur',
    'Please enter an admin remark': 'Veuillez saisir une remarque administrateur',
    'No order details found': 'Aucun détail de commande trouvé',
    'No affiliate records found': 'Aucun enregistrement d’affiliation trouvé',
    'Export CSV': 'Exporter CSV',
    'Last 7 days': '7 derniers jours',
    'Last 30 days': '30 derniers jours',
    'Custom range': 'Période personnalisée'
  },
  ja: {
    'Order Management': '注文管理',
    'Order analytics': '注文分析',
    'Revenue amount': '入金額',
    '30-day revenue': '30日間の入金額',
    'External paid amount': '外部支払額',
    'Mail verification': 'メール照合',
    'Pending verification': '照合待ち',
    'Pending mail': 'メール待ち',
    'Verify now': '今すぐ照合',
    'Verify unfinished orders now': '未完了注文を今すぐ照合',
    'Fetch latest mail': '最新メールを取得',
    'Recheck': '再照合',
    'Checking...': '照合中...',
    'Verified': '照合済み',
    'Not required': '不要',
    'Amount mismatch': '金額不一致',
    'Order number mismatch': '注文番号不一致',
    'Mail parse failed': 'メール解析失敗',
    'Mail fetch failed': 'メール取得失敗',
    'Verification timeout': '照合タイムアウト',
    'Order details': '取引明細',
    'Site amount': '入金額',
    'Mail paid amount': 'メール支払額',
    'Worker order number': 'ショップ注文番号',
    'Affiliate': '紹介報酬',
    'Affiliate statistics': '紹介報酬統計',
    'Users with rewards': '報酬のあるユーザー',
    'Period rewards': '期間内報酬',
    'Total rewards': '累計報酬',
    'Available rewards': '出金可能額',
    'Withdrawn rewards': '出金済み',
    'Withdrawal request': '出金申請',
    'Pending withdrawals': '処理待ち出金',
    'Available without withdrawal': '未申請の出金可能額',
    'Mark as paid': '支払い済みにする',
    'Reject withdrawal': '出金を却下',
    'Source orders': '元注文',
    'Admin remark': '管理者メモ',
    'Please enter an admin remark': '管理者メモを入力してください',
    'No order details found': '取引明細はありません',
    'No affiliate records found': '紹介報酬記録はありません',
    'Export CSV': 'CSV をエクスポート',
    'Last 7 days': '直近7日',
    'Last 30 days': '直近30日',
    'Custom range': 'カスタム期間'
  },
  ru: {
    'Order Management': 'Управление заказами',
    'Order analytics': 'Аналитика заказов',
    'Revenue amount': 'Сумма зачислений',
    '30-day revenue': 'Зачисления за 30 дней',
    'External paid amount': 'Внешняя оплаченная сумма',
    'Mail verification': 'Проверка почты',
    'Pending verification': 'Ожидает проверки',
    'Pending mail': 'Ожидает письмо',
    'Verify now': 'Проверить сейчас',
    'Verify unfinished orders now': 'Проверить незавершённые заказы сейчас',
    'Fetch latest mail': 'Получить последние письма',
    'Recheck': 'Проверить повторно',
    'Checking...': 'Проверка...',
    'Verified': 'Проверено',
    'Not required': 'Не требуется',
    'Amount mismatch': 'Несовпадение суммы',
    'Order number mismatch': 'Несовпадение номера заказа',
    'Mail parse failed': 'Не удалось разобрать письмо',
    'Mail fetch failed': 'Не удалось получить письмо',
    'Verification timeout': 'Время проверки истекло',
    'Order details': 'Детали сделок',
    'Site amount': 'Сумма зачисления',
    'Mail paid amount': 'Оплачено по письму',
    'Worker order number': 'Номер заказа магазина',
    'Affiliate': 'Реферальное вознаграждение',
    'Affiliate statistics': 'Статистика рефералов',
    'Users with rewards': 'Пользователи с вознаграждениями',
    'Period rewards': 'Вознаграждения за период',
    'Total rewards': 'Всего вознаграждений',
    'Available rewards': 'Доступно к выводу',
    'Withdrawn rewards': 'Выведено',
    'Withdrawal request': 'Заявка на вывод',
    'Pending withdrawals': 'Ожидающие выводы',
    'Available without withdrawal': 'Доступно без заявки',
    'Mark as paid': 'Отметить как оплачено',
    'Reject withdrawal': 'Отклонить вывод',
    'Source orders': 'Исходные заказы',
    'Admin remark': 'Комментарий администратора',
    'Please enter an admin remark': 'Введите комментарий администратора',
    'No order details found': 'Детали заказов не найдены',
    'No affiliate records found': 'Реферальные записи не найдены',
    'Export CSV': 'Экспорт CSV',
    'Last 7 days': 'Последние 7 дней',
    'Last 30 days': 'Последние 30 дней',
    'Custom range': 'Пользовательский период'
  },
  vi: {
    'Order Management': 'Quản lý đơn hàng',
    'Order analytics': 'Phân tích đơn hàng',
    'Revenue amount': 'Số tiền ghi nhận',
    '30-day revenue': 'Doanh thu 30 ngày',
    'External paid amount': 'Số tiền thanh toán bên ngoài',
    'Mail verification': 'Đối soát email',
    'Pending verification': 'Chờ đối soát',
    'Pending mail': 'Chờ email',
    'Verify now': 'Đối soát ngay',
    'Verify unfinished orders now': 'Đối soát đơn chưa hoàn tất ngay',
    'Fetch latest mail': 'Lấy email mới nhất',
    'Recheck': 'Đối soát lại',
    'Checking...': 'Đang đối soát...',
    'Verified': 'Đã đối soát',
    'Not required': 'Không áp dụng',
    'Amount mismatch': 'Sai lệch số tiền',
    'Order number mismatch': 'Sai lệch mã đơn',
    'Mail parse failed': 'Không thể phân tích email',
    'Mail fetch failed': 'Không thể lấy email',
    'Verification timeout': 'Hết thời gian đối soát',
    'Order details': 'Chi tiết giao dịch',
    'Site amount': 'Số tiền ghi nhận',
    'Mail paid amount': 'Số tiền trong email',
    'Worker order number': 'Mã đơn cửa hàng',
    'Affiliate': 'Hoa hồng giới thiệu',
    'Affiliate statistics': 'Thống kê hoa hồng',
    'Users with rewards': 'Người dùng có hoa hồng',
    'Period rewards': 'Hoa hồng trong kỳ',
    'Total rewards': 'Tổng hoa hồng',
    'Available rewards': 'Có thể rút',
    'Withdrawn rewards': 'Đã rút',
    'Withdrawal request': 'Yêu cầu rút tiền',
    'Pending withdrawals': 'Yêu cầu rút đang chờ',
    'Available without withdrawal': 'Có thể rút chưa yêu cầu',
    'Mark as paid': 'Đánh dấu đã thanh toán',
    'Reject withdrawal': 'Từ chối rút tiền',
    'Source orders': 'Đơn hàng nguồn',
    'Admin remark': 'Ghi chú quản trị',
    'Please enter an admin remark': 'Vui lòng nhập ghi chú quản trị',
    'No order details found': 'Không có chi tiết đơn hàng',
    'No affiliate records found': 'Không có bản ghi hoa hồng',
    'Export CSV': 'Xuất CSV',
    'Last 7 days': '7 ngày gần đây',
    'Last 30 days': '30 ngày gần đây',
    'Custom range': 'Khoảng thời gian tùy chỉnh'
  }
}
```

Use the script pattern from `/Users/ethan/Documents/yunbay/.agents/skills/i18n-translate/SKILL.md`: load each locale JSON, merge keys, sort alphabetically, write back.

- [ ] **Step 3: Apply translations and run sync**

Run:

```bash
cd /Users/ethan/Documents/yunbay/web/default
node scripts/add-order-management-i18n.mjs
bun run i18n:sync
rm scripts/add-order-management-i18n.mjs
```

Expected: locale files updated and temporary script removed.

- [ ] **Step 4: Verify no missing order-management keys and commit**

Run:

```bash
cd /Users/ethan/Documents/yunbay/web/default
bun run i18n:sync
bun run typecheck
```

Expected: PASS.

Commit:

```bash
cd /Users/ethan/Documents/yunbay
git add web/default/src/i18n/locales/en.json web/default/src/i18n/locales/zh.json web/default/src/i18n/locales/fr.json web/default/src/i18n/locales/ja.json web/default/src/i18n/locales/ru.json web/default/src/i18n/locales/vi.json
git commit -m "feat: add order management translations"
```

---

## Task 11: Add optional IMAP-backed mail import adapter

**Files:**
- Modify: `/Users/ethan/Documents/yunbay/go.mod`
- Modify: `/Users/ethan/Documents/yunbay/go.sum`
- Create: `/Users/ethan/Documents/yunbay/service/ldxp_imap_source.go`
- Create: `/Users/ethan/Documents/yunbay/service/ldxp_imap_source_test.go`
- Modify: `/Users/ethan/Documents/yunbay/service/order_mail_check_job.go`

- [ ] **Step 1: Add dependency using Go modules**

Run:

```bash
cd /Users/ethan/Documents/yunbay
go get github.com/emersion/go-imap/v2 github.com/emersion/go-message
```

Expected: `go.mod` and `go.sum` update. If the repository already has an approved IMAP dependency, use the existing package and do not add a second IMAP library.

- [ ] **Step 2: Write config and disabled-source tests**

Create `/Users/ethan/Documents/yunbay/service/ldxp_imap_source_test.go`:

```go
package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLdxpIMAPConfigFromEnvDisabledWhenMissing(t *testing.T) {
	t.Setenv("LDXP_MAIL_IMAP_HOST", "")
	cfg := LdxpIMAPConfigFromEnv()
	assert.False(t, cfg.Enabled())
}

func TestConfiguredMailSourceFallsBackToStoredSource(t *testing.T) {
	t.Setenv("LDXP_MAIL_IMAP_HOST", "")
	source := ConfiguredLdxpMailSource()
	_, ok := source.(StoredLdxpMailSource)
	assert.True(t, ok)

	_, err := source.FetchRecent(context.Background())
	require.NoError(t, err)
}
```

- [ ] **Step 3: Implement env config and source selection**

Create `/Users/ethan/Documents/yunbay/service/ldxp_imap_source.go`:

```go
package service

import (
	"os"
	"strconv"
)

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
	return LdxpIMAPConfig{Host: os.Getenv("LDXP_MAIL_IMAP_HOST"), Port: port, Username: os.Getenv("LDXP_MAIL_IMAP_USER"), Password: os.Getenv("LDXP_MAIL_IMAP_PASSWORD"), Mailbox: mailbox}
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
```

- [ ] **Step 4: Implement IMAP source**

In `/Users/ethan/Documents/yunbay/service/ldxp_imap_source.go`, implement `LdxpIMAPSource` with:

```go
type LdxpIMAPSource struct {
	cfg LdxpIMAPConfig
}

func NewLdxpIMAPSource(cfg LdxpIMAPConfig) *LdxpIMAPSource {
	return &LdxpIMAPSource{cfg: cfg}
}
```

`FetchRecent(ctx)` should:

1. Connect to `cfg.Host:cfg.Port` over TLS.
2. Login with `cfg.Username` and `cfg.Password`.
3. Select `cfg.Mailbox` read-only.
4. Fetch the latest 200 messages.
5. Extract subject, message id, from, body text.
6. Parse with `ParseLdxpOrderMail`.
7. Upsert `model.LdxpMailEvent` by `RawHash` using GORM `FirstOrCreate`.
8. Return parsed mail events.

When logging errors, never include `cfg.Password`.

- [ ] **Step 5: Switch default runner source to configured source**

In `/Users/ethan/Documents/yunbay/service/order_mail_check_job.go`, change:

```go
var defaultOrderMailCheckRunner = NewOrderMailCheckRunner(StoredLdxpMailSource{})
```

to:

```go
var defaultOrderMailCheckRunner = NewOrderMailCheckRunner(ConfiguredLdxpMailSource())
```

- [ ] **Step 6: Run service tests and commit**

Run:

```bash
cd /Users/ethan/Documents/yunbay
go test ./service -run 'TestLdxpIMAP|TestConfiguredMailSource|TestRunSingleMailCheck|TestParseLdxpOrderMail' -count=1
```

Expected: PASS without real IMAP credentials because missing env falls back to stored source.

Commit:

```bash
git add go.mod go.sum service/ldxp_imap_source.go service/ldxp_imap_source_test.go service/order_mail_check_job.go
git commit -m "feat: add configurable LDXP IMAP mail source"
```

---

## Task 12: Final verification, build, and manual QA

**Files:**
- Review all changed files from previous tasks.

- [ ] **Step 1: Run focused backend tests**

Run:

```bash
cd /Users/ethan/Documents/yunbay
go test ./model ./service ./controller -run 'TestOrderManagement|TestAffiliateWithdrawal|TestParseLdxp|TestVerifyLdxp|TestRunSingleMailCheck|TestRunBatchMailCheck|TestParseOrderManagementRange|TestLdxpIMAP|TestConfiguredMailSource' -count=1
```

Expected: PASS.

- [ ] **Step 2: Run broader backend tests for touched packages**

Run:

```bash
cd /Users/ethan/Documents/yunbay
go test ./model ./service ./controller ./router -count=1
```

Expected: PASS. If `./router` has no tests, Go reports `?` with no test files and exit code 0.

- [ ] **Step 3: Run frontend tests, typecheck, i18n sync, and build**

Run:

```bash
cd /Users/ethan/Documents/yunbay/web/default
bun test src/features/order-management/lib/format.test.ts src/hooks/sidebar-data-model.test.ts
bun run i18n:sync
bun run typecheck
bun run build
```

Expected: all commands exit 0.

- [ ] **Step 4: Run whitespace and status checks**

Run:

```bash
cd /Users/ethan/Documents/yunbay
git diff --check
git status --short
```

Expected: no whitespace errors. `git status --short` should show only intentional feature changes if any are uncommitted, plus the pre-existing unrelated `infra/sub2api/...` modifications if they still exist in the working tree.

- [ ] **Step 5: Manual browser QA**

With the app running at the local URL, verify:

1. Admin users see `Order Management` in the sidebar.
2. Ordinary users do not see the sidebar entry and direct navigation redirects to `/403`.
3. Page defaults to `Last 7 days` and shows a visible `Last 30 days` toggle.
4. Switching to `Last 30 days` refreshes KPI cards, chart, orders, and affiliate stats.
5. Order detail rows show site amount, external paid amount, mail paid amount, worker order number, mail verification status, and affiliate info.
6. `Verify now` starts a mail-check job and disables the row action while checking.
7. `Verify unfinished orders now` starts a batch job without waiting for the 9 PM review time.
8. `¥500.00` site amount + `¥425.00` external paid + `¥425.00` mail paid verifies successfully.
9. `¥10.00` site amount + `¥10.30` external paid + `¥10.30` mail paid verifies successfully.
10. `¥10.00` site amount + `¥10.30` external paid + `¥10.00` mail paid shows `Amount mismatch`.
11. Affiliate stats show users with rewards and pending withdrawal request details.
12. Mark paid and reject actions update withdrawal status and refresh the table.

- [ ] **Step 6: Final commit if the last task produced changes**

If final verification fixes changed files, commit them:

```bash
cd /Users/ethan/Documents/yunbay
git add <intentional feature files only>
git commit -m "fix: stabilize admin order management"
```

Do not stage the unrelated `infra/sub2api/...` files.

---

## Acceptance Checklist Mapping

- Admin menu and route: Task 9, Task 6.
- 7-day/30-day/custom range: Task 5, Task 7.
- Chart above details and larger details area: Task 7.
- Order columns for site amount, external paid, mail paid, worker order number, mail status: Task 7.
- Mail check rule uses worker order number and external paid amount: Task 2 and Task 4.
- Immediate single/batch mail check: Task 4, Task 5, Task 7.
- Affiliate user count, reward totals, withdrawal details, paid/reject actions: Task 3, Task 5, Task 8.
- Six-language i18n: Task 10.
- DB compatibility: Task 1, Task 3.
- Secret handling: Task 11 uses env vars and logs no password.
- Tests/build/manual QA: Task 12.
