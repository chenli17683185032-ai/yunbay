package controller

import (
	"fmt"
	"net/http"
	"sync"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/gin-gonic/gin"
)

type groupRatioOptionsUpdateRequest struct {
	GroupRatio      string `json:"group_ratio"`
	GroupGroupRatio string `json:"group_group_ratio"`
}

type groupRatioOptionsResponse struct {
	GroupRatio      string   `json:"group_ratio"`
	GroupGroupRatio string   `json:"group_group_ratio"`
	PackageGroups   []string `json:"package_groups"`
}

type groupRatioRuntimeApplier func(groupRatio, groupGroupRatio string) error

var groupRatioOptionsMutex sync.Mutex

func respondGroupRatioOptionsError(c *gin.Context, status int, err error) {
	c.JSON(status, gin.H{
		"success": false,
		"message": err.Error(),
	})
}

func normalizedGroupRatioRuntimeReadback() (string, string, error) {
	_, groupRatio, err := ratio_setting.ParseAndNormalizeGroupRatioJSON(ratio_setting.GroupRatio2JSONString())
	if err != nil {
		return "", "", err
	}
	_, groupGroupRatio, err := ratio_setting.ParseAndNormalizeGroupGroupRatioJSON(ratio_setting.GroupGroupRatio2JSONString())
	if err != nil {
		return "", "", err
	}
	return groupRatio, groupGroupRatio, nil
}

func groupRatioOptionsSnapshot() (groupRatioOptionsResponse, error) {
	groupRatio, groupGroupRatio, err := normalizedGroupRatioRuntimeReadback()
	if err != nil {
		return groupRatioOptionsResponse{}, err
	}
	packageGroups, err := model.ListEnabledValuePackageBillingGroups()
	if err != nil {
		return groupRatioOptionsResponse{}, err
	}
	return groupRatioOptionsResponse{
		GroupRatio:      groupRatio,
		GroupGroupRatio: groupGroupRatio,
		PackageGroups:   packageGroups,
	}, nil
}

func setGroupRatioOptionMap(groupRatio, groupGroupRatio string) {
	common.OptionMapRWMutex.Lock()
	defer common.OptionMapRWMutex.Unlock()
	if common.OptionMap == nil {
		common.OptionMap = make(map[string]string)
	}
	common.OptionMap["GroupRatio"] = groupRatio
	common.OptionMap["GroupGroupRatio"] = groupGroupRatio
}

func applyGroupRatioRuntime(groupRatio, groupGroupRatio string) error {
	if err := ratio_setting.UpdateGroupRatioByJSONString(groupRatio); err != nil {
		return err
	}
	return ratio_setting.UpdateGroupGroupRatioByJSONString(groupGroupRatio)
}

func restoreGroupRatioRuntimeFromDatabase() error {
	stored, err := model.GetGroupRatioOptions()
	if err != nil {
		return err
	}
	if err := applyGroupRatioRuntime(stored.GroupRatio, stored.GroupGroupRatio); err != nil {
		return err
	}
	setGroupRatioOptionMap(stored.GroupRatio, stored.GroupGroupRatio)
	return nil
}

func GetGroupRatioOptions(c *gin.Context) {
	groupRatioOptionsMutex.Lock()
	defer groupRatioOptionsMutex.Unlock()

	snapshot, err := groupRatioOptionsSnapshot()
	if err != nil {
		respondGroupRatioOptionsError(c, http.StatusInternalServerError, err)
		return
	}
	common.ApiSuccess(c, snapshot)
}

func UpdateGroupRatioOptions(c *gin.Context) {
	updateGroupRatioOptionsWithRuntime(c, applyGroupRatioRuntime)
}

func updateGroupRatioOptionsWithRuntime(c *gin.Context, applier groupRatioRuntimeApplier) {
	groupRatioOptionsMutex.Lock()
	defer groupRatioOptionsMutex.Unlock()

	var request groupRatioOptionsUpdateRequest
	if err := common.DecodeJson(c.Request.Body, &request); err != nil {
		respondGroupRatioOptionsError(c, http.StatusBadRequest, err)
		return
	}

	_, normalizedGroupRatio, err := ratio_setting.ParseAndNormalizeGroupRatioJSON(request.GroupRatio)
	if err != nil {
		respondGroupRatioOptionsError(c, http.StatusBadRequest, err)
		return
	}
	_, normalizedGroupGroupRatio, err := ratio_setting.ParseAndNormalizeGroupGroupRatioJSON(request.GroupGroupRatio)
	if err != nil {
		respondGroupRatioOptionsError(c, http.StatusBadRequest, err)
		return
	}

	packageGroups, err := model.ListEnabledValuePackageBillingGroups()
	if err != nil {
		respondGroupRatioOptionsError(c, http.StatusInternalServerError, err)
		return
	}
	if err := model.UpdateGroupRatioOptions(normalizedGroupRatio, normalizedGroupGroupRatio); err != nil {
		respondGroupRatioOptionsError(c, http.StatusInternalServerError, err)
		return
	}

	if err := applier(normalizedGroupRatio, normalizedGroupGroupRatio); err != nil {
		if restoreErr := restoreGroupRatioRuntimeFromDatabase(); restoreErr != nil {
			err = fmt.Errorf("apply group ratio runtime: %w; restore from database: %v", err, restoreErr)
		}
		respondGroupRatioOptionsError(c, http.StatusInternalServerError, err)
		return
	}
	setGroupRatioOptionMap(normalizedGroupRatio, normalizedGroupGroupRatio)
	common.ApiSuccess(c, groupRatioOptionsResponse{
		GroupRatio:      normalizedGroupRatio,
		GroupGroupRatio: normalizedGroupGroupRatio,
		PackageGroups:   packageGroups,
	})
}
