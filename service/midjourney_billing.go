package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/model"
)

func refundMidjourneyFunding(task *model.Midjourney) (bool, error) {
	billingContext := task.BillingContext
	if task.FundingRefundQuota() <= 0 {
		return model.MarkMidjourneyFundingRefunded(task)
	}
	if billingContext.BillingSource == BillingSourceSubscription {
		if requestID := strings.TrimSpace(billingContext.RequestId); requestID != "" {
			if err := model.RefundSubscriptionPreConsume(requestID); err != nil {
				return false, err
			}
			return model.MarkMidjourneyFundingRefunded(task)
		}
		if billingContext.SubscriptionId > 0 {
			return model.RefundMidjourneySubscriptionFundingOnce(task)
		}
	}
	return model.RefundMidjourneyWalletFundingOnce(task)
}

func refundMidjourneyToken(ctx context.Context, task *model.Midjourney) (bool, error) {
	tokenID := task.BillingContext.TokenId
	if tokenID <= 0 || task.TokenRefundQuota() <= 0 {
		return false, nil
	}
	tokenKey := resolveTokenKey(ctx, tokenID, task.MjId)
	if tokenKey == "" {
		return false, fmt.Errorf("token key is unavailable")
	}
	return model.RefundMidjourneyTokenQuotaOnce(task, tokenKey)
}

func midjourneyRefundOther(task *model.Midjourney, reason string) map[string]interface{} {
	context := task.BillingContext
	other := map[string]interface{}{
		"task_id": task.MjId,
		"reason":  reason,
	}
	if context.BillingSource != "" {
		other["billing_source"] = context.BillingSource
	}
	if context.EffectiveGroupRatio != 0 {
		other["group_ratio"] = context.EffectiveGroupRatio
	}
	if context.Version >= model.MidjourneyBillingContextVersion {
		other["funding_quota"] = context.FundingQuota
		other["token_quota"] = context.TokenQuota
	}
	if context.ValuePackageSubscriptionId > 0 {
		other["value_package_subscription_id"] = context.ValuePackageSubscriptionId
		other["value_package_billing_group"] = context.ValuePackageBillingGroup
		other["value_package_billing_using_group"] = context.BillingUsingGroup
		other["value_package_effective_ratio"] = context.EffectiveGroupRatio
		other["value_package_ratio_source"] = context.SubscriptionRatioSource
	}
	return other
}

// RefundMidjourneyQuota refunds a terminally failed MJ task using its frozen funding snapshot.
func RefundMidjourneyQuota(ctx context.Context, task *model.Midjourney, reason string) error {
	if task == nil || task.BillingContext.BillingRefunded {
		return nil
	}
	if task.BillingContext.Version < model.MidjourneyBillingContextVersion && task.Quota <= 0 {
		return nil
	}
	fundingPerformed, err := refundMidjourneyFunding(task)
	if err != nil {
		return err
	}
	tokenPerformed, err := refundMidjourneyToken(ctx, task)
	if err != nil {
		return err
	}
	if (!fundingPerformed || task.FundingRefundQuota() <= 0) &&
		(!tokenPerformed || task.TokenRefundQuota() <= 0) {
		return nil
	}
	logQuota := task.FundingRefundQuota()
	if logQuota <= 0 {
		logQuota = task.TokenRefundQuota()
	}
	model.RecordTaskBillingLog(model.RecordTaskBillingLogParams{
		UserId:    task.UserId,
		LogType:   model.LogTypeRefund,
		ChannelId: task.ChannelId,
		ModelName: CovertMjpActionToModelName(task.Action),
		Quota:     logQuota,
		TokenId:   task.BillingContext.TokenId,
		Group:     task.BillingContext.BillingUsingGroup,
		Other:     midjourneyRefundOther(task, reason),
	})
	return nil
}

// CommitMidjourneyTaskUpdate applies a CAS state transition and synchronously
// completes persisted refund legs before exposing the requested final progress.
func CommitMidjourneyTaskUpdate(ctx context.Context, task *model.Midjourney, preStatus string, shouldRefund bool, reason string) (bool, error) {
	if task == nil || task.Id <= 0 {
		return false, fmt.Errorf("midjourney task is nil")
	}
	stored, err := model.GetMidjourneyById(task.Id)
	if err != nil {
		return false, err
	}
	if stored.Progress == "REFUND_PENDING" {
		if stored.FailReason != "" {
			reason = stored.FailReason
		}
		if err := RefundMidjourneyQuota(ctx, stored, reason); err != nil {
			*task = *stored
			return true, err
		}
		completed, err := model.CompleteMidjourneyRefundProgress(stored.Id, "100%")
		if err != nil {
			*task = *stored
			return true, err
		}
		latest, loadErr := model.GetMidjourneyById(stored.Id)
		if loadErr != nil {
			return completed, loadErr
		}
		*task = *latest
		return completed || latest.Progress == "100%", nil
	}
	if stored.Progress == "100%" {
		*task = *stored
		return false, nil
	}
	if stored.Status != preStatus {
		*task = *stored
		return false, nil
	}

	targetProgress := task.Progress
	fromProgress := stored.Progress
	task.BillingContext = stored.BillingContext
	if shouldRefund {
		task.Progress = "REFUND_PENDING"
	}
	won, err := task.UpdateWithStatusAndProgress(preStatus, fromProgress)
	if err != nil || !won || !shouldRefund {
		return won, err
	}
	if err := RefundMidjourneyQuota(ctx, task, reason); err != nil {
		return true, err
	}
	completed, err := model.CompleteMidjourneyRefundProgress(task.Id, targetProgress)
	if err != nil {
		return true, err
	}
	if completed {
		task.Progress = targetProgress
	}
	return completed, nil
}
