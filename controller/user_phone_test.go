package controller

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func performUserMutationRequest(t *testing.T, method string, body string, handler gin.HandlerFunc) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(method, "/api/user/", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set("id", 9999)
	c.Set("role", common.RoleAdminUser)
	c.Set("username", "admin-operator")
	handler(c)
	return recorder
}

func TestCreateUserRejectsUnverifiedPhone(t *testing.T) {
	db := setupManageUserTestDB(t)
	recorder := performUserMutationRequest(
		t,
		http.MethodPost,
		`{"username":"phone-create-user","password":"NewPassword123","display_name":"Phone Create","phone":" 13800138000 ","role":1}`,
		CreateUser,
	)

	assert.NotContains(t, recorder.Body.String(), `"success":true`)
	var count int64
	require.NoError(t, db.Model(&model.User{}).Where("username = ?", "phone-create-user").Count(&count).Error)
	assert.Zero(t, count)
}

func TestUpdateUserRejectsUnverifiedPhoneChange(t *testing.T) {
	db := setupManageUserTestDB(t)
	stored := model.User{
		Username:    "phone-update-user",
		Password:    "stored-password",
		DisplayName: "Before",
		Phone:       "13900139000",
		Role:        common.RoleCommonUser,
		Status:      common.UserStatusEnabled,
		Group:       "default",
		AuthVersion: 1,
	}
	require.NoError(t, db.Create(&stored).Error)

	recorder := performUserMutationRequest(
		t,
		http.MethodPut,
		fmt.Sprintf(
			`{"id":%d,"username":"phone-update-user","display_name":"After","phone":" 13800138000 ","group":"default"}`,
			stored.Id,
		),
		UpdateUser,
	)

	assert.NotContains(t, recorder.Body.String(), `"success":true`)
	var updated model.User
	require.NoError(t, db.First(&updated, stored.Id).Error)
	assert.Equal(t, "13900139000", updated.Phone)
	assert.Equal(t, "Before", updated.DisplayName)
}

func TestUpdateUserPreservesPhoneWhenLegacyClientOmitsField(t *testing.T) {
	db := setupManageUserTestDB(t)
	stored := model.User{
		Username:    "legacy-phone-user",
		Password:    "stored-password",
		DisplayName: "Before",
		Phone:       "13800138000",
		Role:        common.RoleCommonUser,
		Status:      common.UserStatusEnabled,
		Group:       "default",
		AuthVersion: 1,
	}
	require.NoError(t, db.Create(&stored).Error)

	recorder := performUserMutationRequest(
		t,
		http.MethodPut,
		fmt.Sprintf(
			`{"id":%d,"username":"legacy-phone-user","display_name":"After","group":"default"}`,
			stored.Id,
		),
		UpdateUser,
	)

	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.Contains(t, recorder.Body.String(), `"success":true`)

	var updated model.User
	require.NoError(t, db.First(&updated, stored.Id).Error)
	assert.Equal(t, "13800138000", updated.Phone)
}
