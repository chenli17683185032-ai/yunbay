package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/model"
)

func refundMidjourneyFunding(task *model.Midjourney) (bool, error) {
	billingContext := task.BillingContext
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
	if tokenID <= 0 || task.Quota <= 0 {
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
	if task == nil || task.Quota <= 0 || task.BillingContext.BillingRefunded {
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
	if !fundingPerformed && !tokenPerformed {
		return nil
	}
	model.RecordTaskBillingLog(model.RecordTaskBillingLogParams{
		UserId:    task.UserId,
		LogType:   model.LogTypeRefund,
		ChannelId: task.ChannelId,
		ModelName: CovertMjpActionToModelName(task.Action),
		Quota:     task.Quota,
		TokenId:   task.BillingContext.TokenId,
		Group:     task.BillingContext.BillingUsingGroup,
		Other:     midjourneyRefundOther(task, reason),
	})
	return nil
}
