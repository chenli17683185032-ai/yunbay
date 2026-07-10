package model

import (
	"database/sql/driver"
	"fmt"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

type Midjourney struct {
	Id          int    `json:"id"`
	Code        int    `json:"code"`
	UserId      int    `json:"user_id" gorm:"index"`
	Action      string `json:"action" gorm:"type:varchar(40);index"`
	MjId        string `json:"mj_id" gorm:"index"`
	Prompt      string `json:"prompt"`
	PromptEn    string `json:"prompt_en"`
	Description string `json:"description"`
	State       string `json:"state"`
	SubmitTime  int64  `json:"submit_time" gorm:"index"`
	StartTime   int64  `json:"start_time" gorm:"index"`
	FinishTime  int64  `json:"finish_time" gorm:"index"`
	ImageUrl    string `json:"image_url"`
	VideoUrl    string `json:"video_url"`
	VideoUrls   string `json:"video_urls"`
	Status      string `json:"status" gorm:"type:varchar(20);index"`
	Progress    string `json:"progress" gorm:"type:varchar(30);index"`
	FailReason  string `json:"fail_reason"`
	ChannelId   int    `json:"channel_id"`
	Quota       int    `json:"quota"`
	Buttons     string `json:"buttons"`
	Properties  string `json:"properties"`
	// BillingContext is internal settlement/refund state and must not be exposed.
	BillingContext MidjourneyBillingContext `json:"-" gorm:"column:billing_context;type:text"`
}

type MidjourneyBillingContext struct {
	Version                    int     `json:"version,omitempty"`
	BillingSource              string  `json:"billing_source,omitempty"`
	SubscriptionId             int     `json:"subscription_id,omitempty"`
	RequestId                  string  `json:"request_id,omitempty"`
	TokenId                    int     `json:"token_id,omitempty"`
	FundingQuota               int     `json:"funding_quota"`
	TokenQuota                 int     `json:"token_quota"`
	ValuePackageSubscriptionId int     `json:"value_package_subscription_id,omitempty"`
	ValuePackagePlanId         int     `json:"value_package_plan_id,omitempty"`
	ValuePackageModelGroup     string  `json:"value_package_model_group,omitempty"`
	ValuePackagePackageType    string  `json:"value_package_package_type,omitempty"`
	ValuePackageBillingGroup   string  `json:"value_package_billing_group,omitempty"`
	BillingUsingGroup          string  `json:"billing_using_group,omitempty"`
	EffectiveGroupRatio        float64 `json:"effective_group_ratio,omitempty"`
	SubscriptionRatioSource    string  `json:"subscription_ratio_source,omitempty"`
	FundingRefunded            bool    `json:"funding_refunded,omitempty"`
	TokenRefunded              bool    `json:"token_refunded,omitempty"`
	BillingRefunded            bool    `json:"billing_refunded,omitempty"`
}

const MidjourneyBillingContextVersion = 1

func (m *Midjourney) FundingRefundQuota() int {
	if m == nil {
		return 0
	}
	if m.BillingContext.Version >= MidjourneyBillingContextVersion {
		return max(m.BillingContext.FundingQuota, 0)
	}
	return max(m.Quota, 0)
}

func (m *Midjourney) TokenRefundQuota() int {
	if m == nil || m.BillingContext.TokenId <= 0 {
		return 0
	}
	if m.BillingContext.Version >= MidjourneyBillingContextVersion {
		return max(m.BillingContext.TokenQuota, 0)
	}
	return max(m.Quota, 0)
}

func (m *Midjourney) HasRefundableQuota() bool {
	return m != nil && (m.FundingRefundQuota() > 0 || m.TokenRefundQuota() > 0)
}

func (c *MidjourneyBillingContext) Scan(value any) error {
	if value == nil {
		*c = MidjourneyBillingContext{}
		return nil
	}
	var data []byte
	switch typed := value.(type) {
	case []byte:
		data = typed
	case string:
		data = []byte(typed)
	default:
		*c = MidjourneyBillingContext{}
		return nil
	}
	if len(data) == 0 {
		*c = MidjourneyBillingContext{}
		return nil
	}
	return common.Unmarshal(data, c)
}

func (c MidjourneyBillingContext) Value() (driver.Value, error) {
	if c == (MidjourneyBillingContext{}) {
		return nil, nil
	}
	data, err := common.Marshal(c)
	if err != nil {
		return nil, err
	}
	return string(data), nil
}

func loadMidjourneyRefundTask(tx *gorm.DB, taskID int) (*Midjourney, error) {
	var task Midjourney
	if err := withUpdateLock(tx).Where("id = ?", taskID).First(&task).Error; err != nil {
		return nil, err
	}
	return &task, nil
}

func completeMidjourneyFundingContext(context MidjourneyBillingContext, tokenQuota int) MidjourneyBillingContext {
	context.FundingRefunded = true
	if tokenQuota <= 0 {
		context.TokenRefunded = true
		context.BillingRefunded = true
	}
	return context
}

func RefundMidjourneyWalletFundingOnce(task *Midjourney) (bool, error) {
	if task == nil || task.Id <= 0 || task.FundingRefundQuota() <= 0 {
		return false, fmt.Errorf("invalid midjourney wallet refund task")
	}
	var updated MidjourneyBillingContext
	refundQuota := 0
	performed := false
	err := DB.Transaction(func(tx *gorm.DB) error {
		stored, err := loadMidjourneyRefundTask(tx, task.Id)
		if err != nil {
			return err
		}
		if stored.BillingContext.FundingRefunded {
			updated = stored.BillingContext
			return nil
		}
		refundQuota = stored.FundingRefundQuota()
		if refundQuota <= 0 {
			return fmt.Errorf("invalid midjourney wallet refund quota")
		}
		result := tx.Model(&User{}).Where("id = ?", stored.UserId).
			Update("quota", gorm.Expr("quota + ?", refundQuota))
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return gorm.ErrRecordNotFound
		}
		updated = completeMidjourneyFundingContext(stored.BillingContext, stored.TokenRefundQuota())
		if err := tx.Model(&Midjourney{}).Where("id = ?", stored.Id).
			Update("billing_context", updated).Error; err != nil {
			return err
		}
		performed = true
		return nil
	})
	if err != nil {
		return false, err
	}
	task.BillingContext = updated
	if performed && common.RedisEnabled {
		if err := cacheIncrUserQuota(task.UserId, int64(refundQuota)); err != nil {
			common.SysLog("failed to update user quota cache after midjourney refund: " + err.Error())
		}
	}
	return performed, nil
}

func RefundMidjourneySubscriptionFundingOnce(task *Midjourney) (bool, error) {
	if task == nil || task.Id <= 0 || task.FundingRefundQuota() <= 0 || task.BillingContext.SubscriptionId <= 0 {
		return false, fmt.Errorf("invalid midjourney subscription refund task")
	}
	var updated MidjourneyBillingContext
	performed := false
	err := DB.Transaction(func(tx *gorm.DB) error {
		stored, err := loadMidjourneyRefundTask(tx, task.Id)
		if err != nil {
			return err
		}
		if stored.BillingContext.FundingRefunded {
			updated = stored.BillingContext
			return nil
		}
		refundQuota := stored.FundingRefundQuota()
		if refundQuota <= 0 {
			return fmt.Errorf("invalid midjourney subscription refund quota")
		}
		result := tx.Model(&UserSubscription{}).
			Where("id = ?", stored.BillingContext.SubscriptionId).
			Update("amount_used", gorm.Expr("amount_used - ?", refundQuota))
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return gorm.ErrRecordNotFound
		}
		updated = completeMidjourneyFundingContext(stored.BillingContext, stored.TokenRefundQuota())
		if err := tx.Model(&Midjourney{}).Where("id = ?", stored.Id).
			Update("billing_context", updated).Error; err != nil {
			return err
		}
		performed = true
		return nil
	})
	if err != nil {
		return false, err
	}
	task.BillingContext = updated
	return performed, nil
}

func MarkMidjourneyFundingRefunded(task *Midjourney) (bool, error) {
	if task == nil || task.Id <= 0 {
		return false, fmt.Errorf("invalid midjourney refund task")
	}
	var updated MidjourneyBillingContext
	performed := false
	err := DB.Transaction(func(tx *gorm.DB) error {
		stored, err := loadMidjourneyRefundTask(tx, task.Id)
		if err != nil {
			return err
		}
		if stored.BillingContext.FundingRefunded {
			updated = stored.BillingContext
			return nil
		}
		updated = completeMidjourneyFundingContext(stored.BillingContext, stored.TokenRefundQuota())
		if err := tx.Model(&Midjourney{}).Where("id = ?", stored.Id).
			Update("billing_context", updated).Error; err != nil {
			return err
		}
		performed = true
		return nil
	})
	if err != nil {
		return false, err
	}
	task.BillingContext = updated
	return performed, nil
}

func RefundMidjourneyTokenQuotaOnce(task *Midjourney, tokenKey string) (bool, error) {
	if task == nil || task.Id <= 0 || task.TokenRefundQuota() <= 0 || task.BillingContext.TokenId <= 0 {
		return false, fmt.Errorf("invalid midjourney token refund task")
	}
	var updated MidjourneyBillingContext
	performed := false
	err := DB.Transaction(func(tx *gorm.DB) error {
		stored, err := loadMidjourneyRefundTask(tx, task.Id)
		if err != nil {
			return err
		}
		if stored.BillingContext.TokenRefunded {
			updated = stored.BillingContext
			return nil
		}
		refundQuota := stored.TokenRefundQuota()
		if refundQuota <= 0 {
			return fmt.Errorf("invalid midjourney token refund quota")
		}
		result := tx.Model(&Token{}).Where("id = ?", stored.BillingContext.TokenId).Updates(
			map[string]interface{}{
				"remain_quota":  gorm.Expr("remain_quota + ?", refundQuota),
				"used_quota":    gorm.Expr("used_quota - ?", refundQuota),
				"accessed_time": common.GetTimestamp(),
			},
		)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return gorm.ErrRecordNotFound
		}
		updated = stored.BillingContext
		updated.TokenRefunded = true
		updated.BillingRefunded = updated.FundingRefunded
		if err := tx.Model(&Midjourney{}).Where("id = ?", stored.Id).
			Update("billing_context", updated).Error; err != nil {
			return err
		}
		performed = true
		return nil
	})
	if err != nil {
		return false, err
	}
	task.BillingContext = updated
	if performed && common.RedisEnabled && tokenKey != "" {
		if err := cacheDeleteToken(tokenKey); err != nil {
			common.SysLog("failed to invalidate token cache after midjourney refund: " + err.Error())
		}
	}
	return performed, nil
}

// TaskQueryParams 用于包含所有搜索条件的结构体，可以根据需求添加更多字段
type TaskQueryParams struct {
	ChannelID      string
	MjID           string
	StartTimestamp string
	EndTimestamp   string
}

func GetAllUserTask(userId int, startIdx int, num int, queryParams TaskQueryParams) []*Midjourney {
	var tasks []*Midjourney
	var err error

	// 初始化查询构建器
	query := DB.Where("user_id = ?", userId)

	if queryParams.MjID != "" {
		query = query.Where("mj_id = ?", queryParams.MjID)
	}
	if queryParams.StartTimestamp != "" {
		// 假设您已将前端传来的时间戳转换为数据库所需的时间格式，并处理了时间戳的验证和解析
		query = query.Where("submit_time >= ?", queryParams.StartTimestamp)
	}
	if queryParams.EndTimestamp != "" {
		query = query.Where("submit_time <= ?", queryParams.EndTimestamp)
	}

	// 获取数据
	err = query.Order("id desc").Limit(num).Offset(startIdx).Find(&tasks).Error
	if err != nil {
		return nil
	}

	return tasks
}

func GetAllTasks(startIdx int, num int, queryParams TaskQueryParams) []*Midjourney {
	var tasks []*Midjourney
	var err error

	// 初始化查询构建器
	query := DB

	// 添加过滤条件
	if queryParams.ChannelID != "" {
		query = query.Where("channel_id = ?", queryParams.ChannelID)
	}
	if queryParams.MjID != "" {
		query = query.Where("mj_id = ?", queryParams.MjID)
	}
	if queryParams.StartTimestamp != "" {
		query = query.Where("submit_time >= ?", queryParams.StartTimestamp)
	}
	if queryParams.EndTimestamp != "" {
		query = query.Where("submit_time <= ?", queryParams.EndTimestamp)
	}

	// 获取数据
	err = query.Order("id desc").Limit(num).Offset(startIdx).Find(&tasks).Error
	if err != nil {
		return nil
	}

	return tasks
}

func GetAllUnFinishTasks() []*Midjourney {
	var tasks []*Midjourney
	var err error
	// get all tasks progress is not 100%
	err = DB.Where("progress != ?", "100%").Find(&tasks).Error
	if err != nil {
		return nil
	}
	return tasks
}

func GetByOnlyMJId(mjId string) *Midjourney {
	var mj *Midjourney
	var err error
	err = DB.Where("mj_id = ?", mjId).First(&mj).Error
	if err != nil {
		return nil
	}
	return mj
}

func GetByMJId(userId int, mjId string) *Midjourney {
	var mj *Midjourney
	var err error
	err = DB.Where("user_id = ? and mj_id = ?", userId, mjId).First(&mj).Error
	if err != nil {
		return nil
	}
	return mj
}

func GetByMJIds(userId int, mjIds []string) []*Midjourney {
	var mj []*Midjourney
	var err error
	err = DB.Where("user_id = ? and mj_id in (?)", userId, mjIds).Find(&mj).Error
	if err != nil {
		return nil
	}
	return mj
}

func GetMjByuId(id int) *Midjourney {
	var mj *Midjourney
	var err error
	err = DB.Where("id = ?", id).First(&mj).Error
	if err != nil {
		return nil
	}
	return mj
}

func UpdateProgress(id int, progress string) error {
	return DB.Model(&Midjourney{}).Where("id = ?", id).Update("progress", progress).Error
}

func (midjourney *Midjourney) Insert() error {
	var err error
	err = DB.Create(midjourney).Error
	return err
}

func (midjourney *Midjourney) Update() error {
	var err error
	err = DB.Save(midjourney).Error
	return err
}

// UpdateWithStatus performs a conditional UPDATE guarded by fromStatus (CAS).
// Returns (true, nil) if this caller won the update, (false, nil) if
// another process already moved the task out of fromStatus.
// UpdateWithStatus performs a conditional UPDATE guarded by fromStatus (CAS).
// Uses Model().Select("*").Updates() to avoid GORM Save()'s INSERT fallback.
func (midjourney *Midjourney) UpdateWithStatus(fromStatus string) (bool, error) {
	result := DB.Model(midjourney).Where("status = ?", fromStatus).Select("*").Updates(midjourney)
	if result.Error != nil {
		return false, result.Error
	}
	return result.RowsAffected > 0, nil
}

func MjBulkUpdate(mjIds []string, params map[string]any) error {
	return DB.Model(&Midjourney{}).
		Where("mj_id in (?)", mjIds).
		Updates(params).Error
}

func MjBulkUpdateByTaskIds(taskIDs []int, params map[string]any) error {
	return DB.Model(&Midjourney{}).
		Where("id in (?)", taskIDs).
		Updates(params).Error
}

// CountAllTasks returns total midjourney tasks for admin query
func CountAllTasks(queryParams TaskQueryParams) int64 {
	var total int64
	query := DB.Model(&Midjourney{})
	if queryParams.ChannelID != "" {
		query = query.Where("channel_id = ?", queryParams.ChannelID)
	}
	if queryParams.MjID != "" {
		query = query.Where("mj_id = ?", queryParams.MjID)
	}
	if queryParams.StartTimestamp != "" {
		query = query.Where("submit_time >= ?", queryParams.StartTimestamp)
	}
	if queryParams.EndTimestamp != "" {
		query = query.Where("submit_time <= ?", queryParams.EndTimestamp)
	}
	_ = query.Count(&total).Error
	return total
}

// CountAllUserTask returns total midjourney tasks for user
func CountAllUserTask(userId int, queryParams TaskQueryParams) int64 {
	var total int64
	query := DB.Model(&Midjourney{}).Where("user_id = ?", userId)
	if queryParams.MjID != "" {
		query = query.Where("mj_id = ?", queryParams.MjID)
	}
	if queryParams.StartTimestamp != "" {
		query = query.Where("submit_time >= ?", queryParams.StartTimestamp)
	}
	if queryParams.EndTimestamp != "" {
		query = query.Where("submit_time <= ?", queryParams.EndTimestamp)
	}
	_ = query.Count(&total).Error
	return total
}
