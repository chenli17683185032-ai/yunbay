package controller

import (
	"fmt"
	"net/http"

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

var runWithGroupRatioOptionsLock = model.WithGroupRatioOptionsLock

func respondGroupRatioOptionsError(c *gin.Context, status int, err error) {
	c.JSON(status, gin.H{
		"success": false,
		"message": err.Error(),
	})
}

func normalizedGroupRatioRuntimeReadback() (string, string, error) {
	runtimeGroupRatio, runtimeGroupGroupRatio := ratio_setting.GroupRatioPair2JSONStrings()
	_, groupRatio, err := ratio_setting.ParseAndNormalizeGroupRatioJSON(runtimeGroupRatio)
	if err != nil {
		return "", "", err
	}
	_, groupGroupRatio, err := ratio_setting.ParseAndNormalizeGroupGroupRatioJSON(runtimeGroupGroupRatio)
	if err != nil {
		return "", "", err
	}
	return groupRatio, groupGroupRatio, nil
}

func normalizedGroupRatioDatabaseReadback() (model.GroupRatioOptions, error) {
	stored, err := model.GetGroupRatioOptions()
	if err != nil {
		return model.GroupRatioOptions{}, err
	}
	_, groupRatio, err := ratio_setting.ParseAndNormalizeGroupRatioJSON(stored.GroupRatio)
	if err != nil {
		return model.GroupRatioOptions{}, err
	}
	_, groupGroupRatio, err := ratio_setting.ParseAndNormalizeGroupGroupRatioJSON(stored.GroupGroupRatio)
	if err != nil {
		return model.GroupRatioOptions{}, err
	}
	return model.GroupRatioOptions{
		GroupRatio:      groupRatio,
		GroupGroupRatio: groupGroupRatio,
	}, nil
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
	return ratio_setting.UpdateGroupRatioPairByJSONString(groupRatio, groupGroupRatio)
}

func restoreGroupRatioRuntimeFromDatabase() error {
	stored, err := normalizedGroupRatioDatabaseReadback()
	if err != nil {
		return err
	}
	if err := applyGroupRatioRuntime(stored.GroupRatio, stored.GroupGroupRatio); err != nil {
		return err
	}
	setGroupRatioOptionMap(stored.GroupRatio, stored.GroupGroupRatio)
	return nil
}

func restoreGroupRatioRuntimeAfterFailure(operationErr error) error {
	if restoreErr := restoreGroupRatioRuntimeFromDatabase(); restoreErr != nil {
		return fmt.Errorf("group ratio operation: %w; restore from database: %v", operationErr, restoreErr)
	}
	return operationErr
}

func reconcileGroupRatioOptionsReadback(expectedGroupRatio, expectedGroupGroupRatio string) (groupRatioOptionsResponse, error) {
	stored, err := normalizedGroupRatioDatabaseReadback()
	if err != nil {
		return groupRatioOptionsResponse{}, err
	}
	runtimeGroupRatio, runtimeGroupGroupRatio, err := normalizedGroupRatioRuntimeReadback()
	if err != nil {
		return groupRatioOptionsResponse{}, err
	}
	if stored.GroupRatio != expectedGroupRatio || stored.GroupGroupRatio != expectedGroupGroupRatio {
		return groupRatioOptionsResponse{}, fmt.Errorf("committed group ratio options differ from normalized submission")
	}
	if runtimeGroupRatio != stored.GroupRatio || runtimeGroupGroupRatio != stored.GroupGroupRatio {
		return groupRatioOptionsResponse{}, fmt.Errorf("runtime group ratio options differ from committed database snapshot")
	}
	packageGroups, err := model.ListEnabledValuePackageBillingGroups()
	if err != nil {
		return groupRatioOptionsResponse{}, err
	}
	setGroupRatioOptionMap(runtimeGroupRatio, runtimeGroupGroupRatio)
	return groupRatioOptionsResponse{
		GroupRatio:      runtimeGroupRatio,
		GroupGroupRatio: runtimeGroupGroupRatio,
		PackageGroups:   packageGroups,
	}, nil
}

func GetGroupRatioOptions(c *gin.Context) {
	var snapshot groupRatioOptionsResponse
	err := runWithGroupRatioOptionsLock(func() error {
		var err error
		snapshot, err = groupRatioOptionsSnapshot()
		return err
	})
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

	var readback groupRatioOptionsResponse
	err = runWithGroupRatioOptionsLock(func() error {
		if err := model.UpdateGroupRatioOptions(normalizedGroupRatio, normalizedGroupGroupRatio); err != nil {
			return err
		}
		if err := applier(normalizedGroupRatio, normalizedGroupGroupRatio); err != nil {
			return restoreGroupRatioRuntimeAfterFailure(err)
		}
		var err error
		readback, err = reconcileGroupRatioOptionsReadback(normalizedGroupRatio, normalizedGroupGroupRatio)
		if err != nil {
			return restoreGroupRatioRuntimeAfterFailure(err)
		}
		return nil
	})
	if err != nil {
		respondGroupRatioOptionsError(c, http.StatusInternalServerError, err)
		return
	}
	common.ApiSuccess(c, readback)
}
