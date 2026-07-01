package controller

import (
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

func setupRedemptionExportTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	gin.SetMode(gin.TestMode)
	common.UsingSQLite = true
	common.UsingMySQL = false
	common.UsingPostgreSQL = false
	common.RedisEnabled = false

	db, err := gorm.Open(sqlite.Open("file:"+strings.ReplaceAll(t.Name(), "/", "_")+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	model.DB = db
	model.LOG_DB = db

	require.NoError(t, db.AutoMigrate(&model.Redemption{}, &model.User{}, &model.Log{}))

	t.Cleanup(func() {
		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
	})

	return db
}

func newRedemptionExportContext(target string) (*gin.Context, *httptest.ResponseRecorder) {
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, target, nil)
	ctx.Set("id", 1)
	ctx.Set("username", "admin")
	ctx.Set("role", 100)
	return ctx, recorder
}

func seedExportRedemption(t *testing.T, db *gorm.DB, key string, batchID string, status int) model.Redemption {
	t.Helper()
	redemption := model.Redemption{
		UserId:      1,
		Key:         key,
		Status:      status,
		Name:        "Batch A",
		Kind:        "paid_topup",
		Amount:      1000,
		Money:       9.99,
		Quota:       1000,
		BatchId:     batchID,
		Source:      "ldxp",
		CreatedTime: 11,
		ExpiredTime: 22,
	}
	require.NoError(t, db.Create(&redemption).Error)
	return redemption
}

func TestExportRedemptionsRequiresBatchId(t *testing.T) {
	setupRedemptionExportTestDB(t)
	ctx, recorder := newRedemptionExportContext("/api/redemption/export")

	ExportRedemptions(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)
	body := decodeTestResponse(t, recorder)
	require.Equal(t, false, body["success"])
	require.Contains(t, body["message"], "redemption.export_batch_required")
}

func TestExportRedemptionsTxtWritesKeysMarksExportedAndKeepsStatus(t *testing.T) {
	db := setupRedemptionExportTestDB(t)
	first := seedExportRedemption(t, db, "key-a", "batch-1", common.RedemptionCodeStatusEnabled)
	second := seedExportRedemption(t, db, "key-b", "batch-1", common.RedemptionCodeStatusDisabled)
	seedExportRedemption(t, db, "key-other", "batch-2", common.RedemptionCodeStatusEnabled)
	ctx, recorder := newRedemptionExportContext("/api/redemption/export?batch_id=batch-1")

	ExportRedemptions(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Equal(t, "text/plain; charset=utf-8", recorder.Header().Get("Content-Type"))
	require.Equal(t, "key-a\nkey-b\n", recorder.Body.String())

	var exported []model.Redemption
	require.NoError(t, db.Order("id asc").Find(&exported, "batch_id = ?", "batch-1").Error)
	require.Len(t, exported, 2)
	require.Positive(t, exported[0].ExportedTime)
	require.Positive(t, exported[1].ExportedTime)
	require.Equal(t, first.Status, exported[0].Status)
	require.Equal(t, second.Status, exported[1].Status)

	var other model.Redemption
	require.NoError(t, db.First(&other, "batch_id = ?", "batch-2").Error)
	require.Zero(t, other.ExportedTime)
}

func TestExportRedemptionsCsvIncludesHeaderAndRow(t *testing.T) {
	db := setupRedemptionExportTestDB(t)
	seedExportRedemption(t, db, "key-csv", "batch-csv", common.RedemptionCodeStatusEnabled)
	ctx, recorder := newRedemptionExportContext("/api/redemption/export?batch_id=batch-csv&format=csv")

	ExportRedemptions(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Equal(t, "text/csv; charset=utf-8", recorder.Header().Get("Content-Type"))
	lines := strings.Split(strings.TrimSpace(recorder.Body.String()), "\n")
	require.Len(t, lines, 2)
	require.Equal(t, "key,name,kind,amount,money,quota,batch_id,source,expired_time", strings.TrimSuffix(lines[0], "\r"))
	require.Equal(t, "key-csv,Batch A,paid_topup,1000,9.99,1000,batch-csv,ldxp,22", strings.TrimSuffix(lines[1], "\r"))
}
