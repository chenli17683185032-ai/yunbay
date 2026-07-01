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
	err := model.DB.WithContext(ctx).Where("order_no <> ?", "").Order("created_time DESC, id DESC").Limit(500).Find(&events).Error
	return events, err
}
