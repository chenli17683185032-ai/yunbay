package controller

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupAffiliateControllerTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	oldDB := model.DB
	oldLogDB := model.LOG_DB
	oldRedisEnabled := common.RedisEnabled
	oldUsingSQLite := common.UsingSQLite
	oldUsingMySQL := common.UsingMySQL
	oldUsingPostgreSQL := common.UsingPostgreSQL

	gin.SetMode(gin.TestMode)
	common.RedisEnabled = false
	common.UsingSQLite = true
	common.UsingMySQL = false
	common.UsingPostgreSQL = false

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.TopUp{}, &model.AffiliateCommission{}, &model.AffiliateWithdrawal{}))
	model.DB = db
	model.LOG_DB = db

	t.Cleanup(func() {
		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
		model.DB = oldDB
		model.LOG_DB = oldLogDB
		common.RedisEnabled = oldRedisEnabled
		common.UsingSQLite = oldUsingSQLite
		common.UsingMySQL = oldUsingMySQL
		common.UsingPostgreSQL = oldUsingPostgreSQL
	})

	return db
}

func affiliateControllerRequest(t *testing.T, method string, path string, handler gin.HandlerFunc, userID int, body any) *httptest.ResponseRecorder {
	t.Helper()

	router := gin.New()
	router.Handle(method, path, func(c *gin.Context) {
		if userID > 0 {
			c.Set("id", userID)
		}
		handler(c)
	})

	var reqBody bytes.Buffer
	if body != nil {
		payload, err := common.Marshal(body)
		require.NoError(t, err)
		reqBody.Write(payload)
	}
	req := httptest.NewRequest(method, path, &reqBody)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)
	return recorder
}

func insertAffiliateControllerUser(t *testing.T, id int, username string, affCode string, inviterID int) model.User {
	t.Helper()
	user := model.User{
		Id:        id,
		Username:  username,
		Password:  "hash",
		Role:      common.RoleCommonUser,
		Status:    common.UserStatusEnabled,
		Group:     model.UserGroupDefault,
		AffCode:   affCode,
		InviterId: inviterID,
	}
	require.NoError(t, model.DB.Create(&user).Error)
	return user
}

func createAffiliateControllerCommission(t *testing.T, db *gorm.DB, inviterID int, inviteeID int, money float64) model.AffiliateCommission {
	t.Helper()
	insertAffiliateControllerUser(t, inviterID, fmt.Sprintf("affiliate-inviter-%d", inviterID), "api-aff", 0)
	insertAffiliateControllerUser(t, inviteeID, fmt.Sprintf("affiliate-invitee-%d", inviteeID), fmt.Sprintf("invitee-aff-%d", inviteeID), inviterID)

	topUp := model.TopUp{
		UserId:  inviteeID,
		TradeNo: fmt.Sprintf("affiliate-controller-topup-%d", inviteeID),
		Money:   money,
		Amount:  int64(money),
		Status:  common.TopUpStatusSuccess,
	}
	require.NoError(t, model.DB.Create(&topUp).Error)
	require.NoError(t, db.Transaction(func(tx *gorm.DB) error {
		return model.MaybeCreateAffiliateCommissionForTopUpTx(tx, &topUp)
	}))

	var commission model.AffiliateCommission
	require.NoError(t, model.DB.Where("topup_id = ?", topUp.Id).First(&commission).Error)
	return commission
}

func TestCreateAffiliateWithdrawalRejectsMissingContact(t *testing.T) {
	setupAffiliateControllerTestDB(t)
	userID := 701
	insertAffiliateControllerUser(t, userID, "missing-contact-user", "missing-contact-aff", 0)

	response := affiliateControllerRequest(t, http.MethodPost, "/api/user/affiliate/withdrawals", CreateAffiliateWithdrawal, userID, gin.H{
		"amount":  1,
		"contact": "",
	})

	require.Equal(t, http.StatusOK, response.Code)
	require.Contains(t, strings.ToLower(response.Body.String()), "contact")
}

func TestGetAffiliateSummaryReturnsExistingAffCodeAndBalances(t *testing.T) {
	db := setupAffiliateControllerTestDB(t)
	createAffiliateControllerCommission(t, db, 711, 712, 100)

	response := affiliateControllerRequest(t, http.MethodGet, "/api/user/affiliate/summary", GetAffiliateSummary, 711, nil)

	require.Equal(t, http.StatusOK, response.Code)
	body := response.Body.String()
	require.Contains(t, body, "api-aff")
	require.Contains(t, body, "available_money")
	require.Contains(t, body, "15")
}

func TestAffiliateAdminProcessWithdrawalHandlers(t *testing.T) {
	db := setupAffiliateControllerTestDB(t)
	createAffiliateControllerCommission(t, db, 721, 722, 100)
	withdrawal, err := model.CreateAffiliateWithdrawal(721, 10, "admin-process@example.com", "process me")
	require.NoError(t, err)

	path := fmt.Sprintf("/api/user/affiliate/withdrawals/%d/paid", withdrawal.Id)
	router := gin.New()
	router.POST("/api/user/affiliate/withdrawals/:id/paid", MarkAffiliateWithdrawalPaid)
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewBufferString(`{"admin_remark":"done"}`))
	req.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, req)

	require.Equal(t, http.StatusOK, response.Code)
	require.Contains(t, response.Body.String(), model.AffiliateWithdrawalStatusPaid)

	var updated model.AffiliateWithdrawal
	require.NoError(t, model.DB.First(&updated, withdrawal.Id).Error)
	require.Equal(t, model.AffiliateWithdrawalStatusPaid, updated.Status)
	require.Equal(t, "done", updated.AdminRemark)
}
