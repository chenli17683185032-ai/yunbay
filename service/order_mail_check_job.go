package service

import (
	"context"
	"errors"
	"strconv"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
)

const (
	orderMailCheckJobStatusRunning  = "running"
	orderMailCheckJobStatusFinished = "finished"
	orderMailCheckJobStatusFailed   = "failed"

	orderMailCheckJobTimeoutSeconds   = 30
	orderMailCheckJobRetentionSeconds = 600
	orderMailCheckMaxJobs             = 1000
)

var defaultOrderMailCheckRunner = NewOrderMailCheckRunner(ConfiguredLdxpMailSource())

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
	return defaultOrderMailCheckRunner
}

func (r *OrderMailCheckRunner) StartSingle(ctx context.Context, sessionId int) *OrderMailCheckJobStatus {
	job := r.createJob()
	go func() {
		jobCtx, cancel := context.WithTimeout(context.Background(), time.Duration(orderMailCheckJobTimeoutSeconds)*time.Second)
		defer cancel()
		result := r.RunSingle(jobCtx, sessionId)
		r.finishJob(job.JobId, result)
	}()
	return job
}

func (r *OrderMailCheckRunner) StartBatch(ctx context.Context, filter model.OrderMailCheckBatchFilter) *OrderMailCheckJobStatus {
	job := r.createJob()
	go func() {
		jobCtx, cancel := context.WithTimeout(context.Background(), time.Duration(orderMailCheckJobTimeoutSeconds)*time.Second)
		defer cancel()
		result := r.RunBatch(jobCtx, filter)
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
	deleted, err := model.IsOrderManagementSessionDeleted(session)
	if err != nil {
		return OrderMailCheckResult{Error: err}
	}
	if deleted {
		return OrderMailCheckResult{}
	}
	return r.verifySessions(ctx, []model.LdxpTopupSession{session})
}

func (r *OrderMailCheckRunner) RunBatch(ctx context.Context, filter model.OrderMailCheckBatchFilter) OrderMailCheckResult {
	sessions, err := model.ListMailCheckCandidatesWithContext(ctx, filter)
	if err != nil {
		return OrderMailCheckResult{Error: err}
	}
	return r.verifySessions(ctx, sessions)
}

func (r *OrderMailCheckRunner) createJob() *OrderMailCheckJobStatus {
	now := common.GetTimestamp()
	job := &OrderMailCheckJobStatus{
		Status:      orderMailCheckJobStatusRunning,
		CreatedTime: now,
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	r.cleanupJobsLocked(now)
	id := time.Now().UnixNano()
	for {
		job.JobId = strconv.FormatInt(id, 10)
		if _, exists := r.jobs[job.JobId]; !exists {
			break
		}
		id++
	}
	r.jobs[job.JobId] = job
	return job
}

func (r *OrderMailCheckRunner) cleanupJobsLocked(now int64) {
	if len(r.jobs) == 0 {
		return
	}
	for jobId, job := range r.jobs {
		if job.Status == orderMailCheckJobStatusRunning {
			continue
		}
		if job.FinishedTime > 0 && now-job.FinishedTime >= orderMailCheckJobRetentionSeconds {
			delete(r.jobs, jobId)
		}
	}
	if len(r.jobs) <= orderMailCheckMaxJobs {
		return
	}
	for jobId, job := range r.jobs {
		if len(r.jobs) <= orderMailCheckMaxJobs {
			return
		}
		if job.Status != orderMailCheckJobStatusRunning {
			delete(r.jobs, jobId)
		}
	}
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
		affected, updateErr := r.markFetchFailed(ctx, sessions, err)
		return OrderMailCheckResult{AffectedCount: affected, Error: errors.Join(err, updateErr)}
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
	db := model.DB.WithContext(ctx)
	for _, session := range sessions {
		affected++
		if err := db.Model(&model.LdxpTopupSession{}).Where("id = ?", session.Id).Updates(map[string]interface{}{
			"error_code":    model.MailCheckStatusChecking,
			"error_message": "邮件核对中",
			"updated_time":  common.GetTimestamp(),
		}).Error; err != nil {
			return OrderMailCheckResult{AffectedCount: affected, Error: err}
		}

		mail := mailByOrder[session.WorkerOrderNo]
		if mail == nil {
			if err := db.Model(&model.LdxpTopupSession{}).Where("id = ?", session.Id).Updates(map[string]interface{}{
				"error_code":    "waiting_mail",
				"error_message": "未找到匹配订单确认邮件",
				"updated_time":  common.GetTimestamp(),
			}).Error; err != nil {
				return OrderMailCheckResult{AffectedCount: affected, Error: err}
			}
			continue
		}

		parsedMail := &ParsedLdxpMail{
			ProductName:   mail.ProductName,
			PaidCents:     orderManagementParsedMailCents(mail.Amount),
			PaymentTime:   mail.PaidTime,
			OrderNo:       mail.OrderNo,
			ContentMasked: mail.BodyExcerpt,
		}
		verification := VerifyLdxpMail(&session, parsedMail)
		messageID := ""
		if mail.MessageId != nil {
			messageID = *mail.MessageId
		}
		updates := map[string]interface{}{
			"mail_message_id":    messageID,
			"mail_order_no":      mail.OrderNo,
			"mail_amount":        mail.Amount,
			"mail_product_name":  mail.ProductName,
			"mail_card_key":      mail.CardKey,
			"mail_from":          mail.MailFrom,
			"mail_to":            mail.MailTo,
			"mail_subject":       mail.Subject,
			"mail_received_time": mail.ReceivedTime,
			"status":             ldxpStatusFromMailCheckStatus(verification.Status),
			"error_code":         verification.ErrorCode,
			"error_message":      verification.ErrorMessage,
			"updated_time":       common.GetTimestamp(),
		}
		if verification.Status == model.MailCheckStatusVerified {
			updates["verified_time"] = common.GetTimestamp()
			updates["error_code"] = ""
			updates["error_message"] = ""
		}
		if err := db.Model(&model.LdxpTopupSession{}).Where("id = ?", session.Id).Updates(updates).Error; err != nil {
			return OrderMailCheckResult{AffectedCount: affected, Error: err}
		}
	}
	return OrderMailCheckResult{AffectedCount: affected}
}

func (r *OrderMailCheckRunner) markFetchFailed(ctx context.Context, sessions []model.LdxpTopupSession, fetchErr error) (int, error) {
	affected := 0
	db := model.DB.WithContext(ctx)
	for _, session := range sessions {
		result := db.Model(&model.LdxpTopupSession{}).Where("id = ?", session.Id).Updates(map[string]interface{}{
			"status":        model.LdxpStatusVerifyFailed,
			"error_code":    "mail_fetch_failed",
			"error_message": fetchErr.Error(),
			"updated_time":  common.GetTimestamp(),
		})
		if result.Error != nil {
			return affected, result.Error
		}
		affected++
	}
	return affected, nil
}

func ldxpStatusFromMailCheckStatus(status string) string {
	switch status {
	case model.MailCheckStatusVerified:
		return model.LdxpStatusVerified
	case model.MailCheckStatusWaitingMail:
		return model.LdxpStatusWorkerPaid
	case model.MailCheckStatusTimeout:
		return model.LdxpStatusMailTimeout
	case model.MailCheckStatusAmountMismatch, model.MailCheckStatusOrderMismatch, model.MailCheckStatusMailParseFailed, model.MailCheckStatusMailFetchFailed:
		return model.LdxpStatusVerifyFailed
	default:
		return model.LdxpStatusVerifyFailed
	}
}
