package service

import (
	"context"
	"errors"
	"testing"
	"time"

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

func TestRunSingleMailCheckFetchErrorMarksSessionFailed(t *testing.T) {
	model.TruncateOrderManagementTablesForTest(t)

	session := &model.LdxpTopupSession{SessionId: "job_fetch_failed", UserId: 1, ExternalPaidCents: 1030, WorkerOrderNo: "LD260628FETCH", MailStatus: model.MailCheckStatusPending, CreatedTime: 1782600000}
	require.NoError(t, model.DB.Create(session).Error)

	job := NewOrderMailCheckRunner(fakeMailSource{err: errors.New("imap unavailable")})
	result := job.RunSingle(context.Background(), session.Id)
	require.Error(t, result.Error)
	assert.Equal(t, 1, result.AffectedCount)

	var saved model.LdxpTopupSession
	require.NoError(t, model.DB.First(&saved, session.Id).Error)
	assert.Equal(t, model.MailCheckStatusMailFetchFailed, saved.MailStatus)
	assert.Equal(t, "mail_fetch_failed", saved.ErrorCode)
	assert.Equal(t, "imap unavailable", saved.ErrorMessage)
}

func TestRunSingleMailCheckNoMatchingMailMarksWaiting(t *testing.T) {
	model.TruncateOrderManagementTablesForTest(t)

	session := &model.LdxpTopupSession{SessionId: "job_waiting", UserId: 1, ExternalPaidCents: 1030, WorkerOrderNo: "LD260628WAIT", MailStatus: model.MailCheckStatusPending, CreatedTime: 1782600000}
	require.NoError(t, model.DB.Create(session).Error)
	mail := &model.LdxpMailEvent{OrderNo: "LD260628OTHER", PaidCents: 1030, ParseStatus: "parsed", CreatedTime: 1782600001}

	job := NewOrderMailCheckRunner(fakeMailSource{mails: []*model.LdxpMailEvent{mail}})
	result := job.RunSingle(context.Background(), session.Id)
	require.NoError(t, result.Error)
	assert.Equal(t, 1, result.AffectedCount)

	var saved model.LdxpTopupSession
	require.NoError(t, model.DB.First(&saved, session.Id).Error)
	assert.Equal(t, model.MailCheckStatusWaitingMail, saved.MailStatus)
	assert.Equal(t, "waiting_mail", saved.ErrorCode)
	assert.Equal(t, "未找到匹配订单确认邮件", saved.ErrorMessage)
}

func TestStartSingleMailCheckJobEventuallyFinishes(t *testing.T) {
	model.TruncateOrderManagementTablesForTest(t)

	session := &model.LdxpTopupSession{SessionId: "job_async", UserId: 1, ExternalPaidCents: 1030, WorkerOrderNo: "LD260628ASYNC", MailStatus: model.MailCheckStatusPending, CreatedTime: 1782600000}
	require.NoError(t, model.DB.Create(session).Error)
	mail := &model.LdxpMailEvent{OrderNo: "LD260628ASYNC", PaidCents: 1030, ParseStatus: "parsed", CreatedTime: 1782600001}

	job := NewOrderMailCheckRunner(fakeMailSource{mails: []*model.LdxpMailEvent{mail}})
	status := job.StartSingle(context.Background(), session.Id)
	require.NotEmpty(t, status.JobId)
	assert.Equal(t, "running", status.Status)

	deadline := time.Now().Add(2 * time.Second)
	var savedStatus *OrderMailCheckJobStatus
	for time.Now().Before(deadline) {
		current, ok := job.GetJob(status.JobId)
		require.True(t, ok)
		if current.Status == "finished" {
			savedStatus = current
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	require.NotNil(t, savedStatus, "job did not finish before timeout")
	assert.Equal(t, 1, savedStatus.AffectedCount)
	assert.Empty(t, savedStatus.ErrorMessage)

	var saved model.LdxpTopupSession
	require.NoError(t, model.DB.First(&saved, session.Id).Error)
	assert.Equal(t, model.MailCheckStatusVerified, saved.MailStatus)
}
