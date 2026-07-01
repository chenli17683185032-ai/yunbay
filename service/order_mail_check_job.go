package service

import (
	"context"
	"strconv"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"gorm.io/gorm"
)

const (
	orderMailCheckJobStatusRunning  = "running"
	orderMailCheckJobStatusFinished = "finished"
	orderMailCheckJobStatusFailed   = "failed"
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
	ErrorMessage  string `json:"error_message"`
	CreatedTime   int64  `json:"created_time"`
	FinishedTime  int64  `json:"finished_time"`
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
	return &OrderMailCheckRunner{source: source, jobs: make(map[string]*OrderMailCheckJobStatus)}
}

func DefaultOrderMailCheckRunner() *OrderMailCheckRunner {
	return NewOrderMailCheckRunner(StoredLdxpMailSource{})
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
		return OrderMailCheckResult{Error: err}
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

func (r *OrderMailCheckRunner) createJob() *OrderMailCheckJobStatus {
	job := &OrderMailCheckJobStatus{
		JobId:       strconv.FormatInt(time.Now().UnixNano(), 10),
		Status:      orderMailCheckJobStatusRunning,
		CreatedTime: common.GetTimestamp(),
	}
	r.mu.Lock()
	r.jobs[job.JobId] = job
	r.mu.Unlock()
	return job
}

func (r *OrderMailCheckRunner) finishJob(jobId string, result OrderMailCheckResult) {
	r.mu.Lock()
	defer r.mu.Unlock()
	job, ok := r.jobs[jobId]
	if !ok {
		return
	}
	job.AffectedCount = result.AffectedCount
	job.FinishedTime = common.GetTimestamp()
	if result.Error != nil {
		job.Status = orderMailCheckJobStatusFailed
		job.ErrorMessage = result.Error.Error()
		return
	}
	job.Status = orderMailCheckJobStatusFinished
}

func (r *OrderMailCheckRunner) verifySessions(ctx context.Context, sessions []model.LdxpTopupSession) OrderMailCheckResult {
	if len(sessions) == 0 {
		return OrderMailCheckResult{}
	}
	mails, err := r.source.FetchRecent(ctx)
	if err != nil {
		return OrderMailCheckResult{AffectedCount: r.markFetchFailed(ctx, sessions, err), Error: err}
	}

	mailByOrder := make(map[string]*model.LdxpMailEvent)
	for _, mail := range mails {
		if mail == nil || mail.OrderNo == "" {
			continue
		}
		if _, ok := mailByOrder[mail.OrderNo]; !ok {
			mailByOrder[mail.OrderNo] = mail
		}
	}

	affected := 0
	for _, session := range sessions {
		affected++
		db := model.DB.WithContext(ctx)
		db.Model(&model.LdxpTopupSession{}).Where("id = ?", session.Id).Updates(map[string]interface{}{
			"mail_status": model.MailCheckStatusChecking,
		})

		mail := mailByOrder[session.WorkerOrderNo]
		if mail == nil {
			db.Model(&model.LdxpTopupSession{}).Where("id = ?", session.Id).Updates(map[string]interface{}{
				"mail_status":   model.MailCheckStatusWaitingMail,
				"error_code":    "waiting_mail",
				"error_message": "未找到匹配订单确认邮件",
			})
			continue
		}

		parsedMail := &ParsedLdxpMail{
			ProductName:   mail.ProductName,
			PaidCents:     mail.PaidCents,
			Quantity:      mail.Quantity,
			PaymentTime:   mail.PaymentTime,
			OrderNo:       mail.OrderNo,
			ContentMasked: mail.ContentMasked,
		}
		verification := VerifyLdxpMail(&session, parsedMail)
		updates := map[string]interface{}{
			"mail_order_no":     mail.OrderNo,
			"mail_amount_cents": mail.PaidCents,
			"mail_event_id":     mail.Id,
			"mail_status":       verification.Status,
			"error_code":        verification.ErrorCode,
			"error_message":     verification.ErrorMessage,
		}
		if verification.Status == model.MailCheckStatusVerified {
			updates["verified_time"] = common.GetTimestamp()
		}
		db.Model(&model.LdxpTopupSession{}).Where("id = ?", session.Id).Updates(updates)
	}
	return OrderMailCheckResult{AffectedCount: affected}
}

func (r *OrderMailCheckRunner) markFetchFailed(ctx context.Context, sessions []model.LdxpTopupSession, fetchErr error) int {
	affected := 0
	db := model.DB.WithContext(ctx)
	for _, session := range sessions {
		result := db.Model(&model.LdxpTopupSession{}).Where("id = ?", session.Id).Updates(map[string]interface{}{
			"mail_status":   model.MailCheckStatusMailFetchFailed,
			"error_code":    "mail_fetch_failed",
			"error_message": fetchErr.Error(),
		})
		if result.Error == nil || result.Error == gorm.ErrRecordNotFound {
			affected++
		}
	}
	return affected
}
