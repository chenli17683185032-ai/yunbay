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

	session := &model.LdxpTopupSession{SessionId: "job_s1", UserId: 1, Money: 10, WorkerAmount: 10.30, WorkerOrderNo: "LD260628UZJ97P", Status: model.LdxpStatusWorkerPaid, CreatedTime: 1782600000}
	require.NoError(t, model.DB.Create(session).Error)
	mail := &model.LdxpMailEvent{RawHash: "hash_job_s1", OrderNo: "LD260628UZJ97P", Amount: 10.30, CreatedTime: 1782600001}
	require.NoError(t, model.DB.Create(mail).Error)

	job := NewOrderMailCheckRunner(fakeMailSource{mails: []*model.LdxpMailEvent{mail}})
	result := job.RunSingle(context.Background(), session.Id)
	require.NoError(t, result.Error)
	assert.Equal(t, 1, result.AffectedCount)

	var saved model.LdxpTopupSession
	require.NoError(t, model.DB.First(&saved, session.Id).Error)
	assert.Equal(t, model.MailCheckStatusVerified, model.OrderManagementMailStatusFromSession(saved))
	assert.Equal(t, model.LdxpStatusVerified, saved.Status)
	assert.Equal(t, 10.30, saved.MailAmount)
}

func TestRunBatchMailCheckHonorsLimit(t *testing.T) {
	model.TruncateOrderManagementTablesForTest(t)
	for i := 0; i < 3; i++ {
		require.NoError(t, model.DB.Create(&model.LdxpTopupSession{SessionId: "batch_s" + string(rune('1'+i)), UserId: 1, WorkerAmount: 1, WorkerOrderNo: "LD260628B" + string(rune('1'+i)), Status: model.LdxpStatusWorkerPaid, CreatedTime: 1782600000 + int64(i)}).Error)
	}
	job := NewOrderMailCheckRunner(fakeMailSource{})
	result := job.RunBatch(context.Background(), model.OrderMailCheckBatchFilter{StartTime: 1782600000, EndTime: 1782609999, Limit: 2})
	require.NoError(t, result.Error)
	assert.Equal(t, 2, result.AffectedCount)
}

func TestRunSingleMailCheckFetchErrorMarksSessionFailed(t *testing.T) {
	model.TruncateOrderManagementTablesForTest(t)

	session := &model.LdxpTopupSession{SessionId: "job_fetch_failed", UserId: 1, WorkerAmount: 10.30, WorkerOrderNo: "LD260628FETCH", Status: model.LdxpStatusWorkerPaid, CreatedTime: 1782600000}
	require.NoError(t, model.DB.Create(session).Error)

	job := NewOrderMailCheckRunner(fakeMailSource{err: errors.New("imap unavailable")})
	result := job.RunSingle(context.Background(), session.Id)
	require.Error(t, result.Error)
	assert.Equal(t, 1, result.AffectedCount)

	var saved model.LdxpTopupSession
	require.NoError(t, model.DB.First(&saved, session.Id).Error)
	assert.Equal(t, model.MailCheckStatusMailFetchFailed, model.OrderManagementMailStatusFromSession(saved))
	assert.Equal(t, model.LdxpStatusVerifyFailed, saved.Status)
	assert.Equal(t, "mail_fetch_failed", saved.ErrorCode)
	assert.Equal(t, "imap unavailable", saved.ErrorMessage)
}

func TestRunSingleMailCheckNoMatchingMailMarksWaiting(t *testing.T) {
	model.TruncateOrderManagementTablesForTest(t)

	session := &model.LdxpTopupSession{SessionId: "job_waiting", UserId: 1, WorkerAmount: 10.30, WorkerOrderNo: "LD260628WAIT", Status: model.LdxpStatusWorkerPaid, CreatedTime: 1782600000}
	require.NoError(t, model.DB.Create(session).Error)
	mail := &model.LdxpMailEvent{OrderNo: "LD260628OTHER", Amount: 10.30, CreatedTime: 1782600001}

	job := NewOrderMailCheckRunner(fakeMailSource{mails: []*model.LdxpMailEvent{mail}})
	result := job.RunSingle(context.Background(), session.Id)
	require.NoError(t, result.Error)
	assert.Equal(t, 1, result.AffectedCount)

	var saved model.LdxpTopupSession
	require.NoError(t, model.DB.First(&saved, session.Id).Error)
	assert.Equal(t, model.MailCheckStatusWaitingMail, model.OrderManagementMailStatusFromSession(saved))
	assert.Equal(t, "waiting_mail", saved.ErrorCode)
	assert.Equal(t, "未找到匹配订单确认邮件", saved.ErrorMessage)
}

func TestStartSingleMailCheckJobEventuallyFinishes(t *testing.T) {
	model.TruncateOrderManagementTablesForTest(t)

	session := &model.LdxpTopupSession{SessionId: "job_async", UserId: 1, WorkerAmount: 10.30, WorkerOrderNo: "LD260628ASYNC", Status: model.LdxpStatusWorkerPaid, CreatedTime: 1782600000}
	require.NoError(t, model.DB.Create(session).Error)
	mail := &model.LdxpMailEvent{OrderNo: "LD260628ASYNC", Amount: 10.30, CreatedTime: 1782600001}

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
	assert.Equal(t, model.MailCheckStatusVerified, model.OrderManagementMailStatusFromSession(saved))
}

func TestDefaultOrderMailCheckRunnerSharesJobsAcrossCalls(t *testing.T) {
	model.TruncateOrderManagementTablesForTest(t)

	session := &model.LdxpTopupSession{SessionId: "job_default", UserId: 1, WorkerAmount: 10.30, WorkerOrderNo: "LD260628DEFAULT", Status: model.LdxpStatusWorkerPaid, CreatedTime: 1782600000}
	require.NoError(t, model.DB.Create(session).Error)
	require.NoError(t, model.DB.Create(&model.LdxpMailEvent{RawHash: "hash_job_default", OrderNo: "LD260628DEFAULT", Amount: 10.30, CreatedTime: 1782600001}).Error)

	status := DefaultOrderMailCheckRunner().StartSingle(context.Background(), session.Id)

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		current, ok := DefaultOrderMailCheckRunner().GetJob(status.JobId)
		if ok && current.Status == "finished" {
			assert.Equal(t, 1, current.AffectedCount)
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("default runner did not share finished job %s across calls", status.JobId)
}

func TestRunSingleMailCheckSetsVerifiedTime(t *testing.T) {
	model.TruncateOrderManagementTablesForTest(t)

	session := &model.LdxpTopupSession{SessionId: "job_verified_time", UserId: 1, WorkerAmount: 10.30, WorkerOrderNo: "LD260628VT", Status: model.LdxpStatusWorkerPaid, CreatedTime: 1782600000}
	require.NoError(t, model.DB.Create(session).Error)
	mail := &model.LdxpMailEvent{OrderNo: "LD260628VT", Amount: 10.30, CreatedTime: 1782600001}

	job := NewOrderMailCheckRunner(fakeMailSource{mails: []*model.LdxpMailEvent{mail}})
	result := job.RunSingle(context.Background(), session.Id)
	require.NoError(t, result.Error)

	var saved model.LdxpTopupSession
	require.NoError(t, model.DB.First(&saved, session.Id).Error)
	assert.Equal(t, model.MailCheckStatusVerified, model.OrderManagementMailStatusFromSession(saved))
	assert.Greater(t, saved.VerifiedTime, int64(0))
}

func TestStoredLdxpMailSourceFetchRecentReturnsEventsWithOrderNoInStableOrder(t *testing.T) {
	model.TruncateOrderManagementTablesForTest(t)

	events := []*model.LdxpMailEvent{
		{RawHash: "hash_old", OrderNo: "LD_OLD", Amount: 1, CreatedTime: 1782600000},
		{RawHash: "hash_new_low", OrderNo: "LD_NEW_LOW_ID", Amount: 1, CreatedTime: 1782600002},
		{RawHash: "hash_ignored", OrderNo: "", Amount: 1, CreatedTime: 1782600003},
		{RawHash: "hash_new_high", OrderNo: "LD_NEW_HIGH_ID", Amount: 1, CreatedTime: 1782600002},
	}
	for _, event := range events {
		require.NoError(t, model.DB.Create(event).Error)
	}

	mails, err := (StoredLdxpMailSource{}).FetchRecent(context.Background())
	require.NoError(t, err)
	require.Len(t, mails, 3)
	assert.Equal(t, "LD_NEW_HIGH_ID", mails[0].OrderNo)
	assert.Equal(t, "LD_NEW_LOW_ID", mails[1].OrderNo)
	assert.Equal(t, "LD_OLD", mails[2].OrderNo)
}

func TestRunBatchMailCheckProcessesNewestCandidatesByCreatedTimeAndID(t *testing.T) {
	model.TruncateOrderManagementTablesForTest(t)

	sessions := []*model.LdxpTopupSession{
		{SessionId: "batch_old", UserId: 1, WorkerAmount: 1, WorkerOrderNo: "LD_BATCH_OLD", Status: model.LdxpStatusWorkerPaid, CreatedTime: 1782600000},
		{SessionId: "batch_new_low_id", UserId: 1, WorkerAmount: 1, WorkerOrderNo: "LD_BATCH_NEW_LOW", Status: model.LdxpStatusWorkerPaid, CreatedTime: 1782600002},
		{SessionId: "batch_new_high_id", UserId: 1, WorkerAmount: 1, WorkerOrderNo: "LD_BATCH_NEW_HIGH", Status: model.LdxpStatusWorkerPaid, CreatedTime: 1782600002},
	}
	for _, session := range sessions {
		require.NoError(t, model.DB.Create(session).Error)
	}

	job := NewOrderMailCheckRunner(fakeMailSource{})
	result := job.RunBatch(context.Background(), model.OrderMailCheckBatchFilter{StartTime: 1782600000, EndTime: 1782609999, Limit: 2})
	require.NoError(t, result.Error)
	assert.Equal(t, 2, result.AffectedCount)

	var saved []model.LdxpTopupSession
	require.NoError(t, model.DB.Order("created_time ASC, id ASC").Find(&saved).Error)
	require.Len(t, saved, 3)
	assert.Empty(t, saved[0].ErrorCode)
	assert.Equal(t, "waiting_mail", saved[1].ErrorCode)
	assert.Equal(t, "waiting_mail", saved[2].ErrorCode)
}

func TestStartSingleMailCheckIgnoresCanceledRequestContext(t *testing.T) {
	model.TruncateOrderManagementTablesForTest(t)

	session := &model.LdxpTopupSession{SessionId: "job_canceled_request", UserId: 1, WorkerAmount: 10.30, WorkerOrderNo: "LD260628CANCEL", Status: model.LdxpStatusWorkerPaid, CreatedTime: 1782600000}
	require.NoError(t, model.DB.Create(session).Error)
	mail := &model.LdxpMailEvent{OrderNo: "LD260628CANCEL", Amount: 10.30, CreatedTime: 1782600001}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	job := NewOrderMailCheckRunner(fakeMailSource{mails: []*model.LdxpMailEvent{mail}})
	status := job.StartSingle(ctx, session.Id)

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		current, ok := job.GetJob(status.JobId)
		require.True(t, ok)
		if current.Status == "finished" {
			assert.Equal(t, 1, current.AffectedCount)
			return
		}
		if current.Status == "failed" {
			t.Fatalf("job used canceled request context: %s", current.ErrorMessage)
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("job did not finish before timeout")
}
