package controller

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service/authz"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupBillingPermissions(t *testing.T, db *gorm.DB) {
	t.Helper()
	require.NoError(t, db.AutoMigrate(&model.CasbinRule{}, &model.AuthzRole{}))
	wasMaster := common.IsMasterNode
	common.IsMasterNode = true
	t.Cleanup(func() { common.IsMasterNode = wasMaster })
	require.NoError(t, authz.Init(db))
}

func TestBillingAccountSearchReturnsFormalIdentityAndSupportsCompanyTitle(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	savedDB := model.DB
	model.DB = db
	t.Cleanup(func() { model.DB = savedDB })
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.BillingAccount{}))
	setupBillingPermissions(t, db)
	for _, user := range []model.User{
		{Id: 1, Username: "plain", AffCode: "plain-code", Password: "private-password", Email: "private-email@example.com"},
		{Id: 2, Username: "draft", AffCode: "draft-code"},
		{Id: 3, Username: "formal", AffCode: "formal-code"},
	} {
		require.NoError(t, db.Create(&user).Error)
	}
	require.NoError(t, db.Create(&model.BillingAccount{UserID: 2, CompanyTitle: "Draft title"}).Error)
	require.NoError(t, db.Create(&model.BillingAccount{UserID: 3, CompanyTitle: "Acme Ltd", AccountingStartAt: 100}).Error)
	for _, scenario := range []struct {
		name, query  string
		role, status int
		ids          []int
		total        int
	}{
		{"all accounts including unconfigured", "?page_size=30", common.RoleAdminUser, 200, []int{3, 2, 1}, 3},
		{"company title", "?keyword=Acme", common.RoleAdminUser, 200, []int{3}, 1},
		{"username", "?keyword=draft", common.RoleAdminUser, 200, []int{2}, 1},
		{"user ID", "?keyword=1", common.RoleAdminUser, 200, []int{1}, 1},
		{"pagination", "?page_size=1&p=2", common.RoleAdminUser, 200, []int{2}, 3},
		{"no match", "?keyword=absent", common.RoleAdminUser, 200, []int{}, 0},
		{"customer forbidden", "", common.RoleCommonUser, 403, nil, 0},
		{"invalid page size", "?page_size=-1", common.RoleAdminUser, 400, nil, 0},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			ctx, _ := gin.CreateTestContext(recorder)
			ctx.Set("role", scenario.role)
			ctx.Set("id", 1)
			ctx.Request = httptest.NewRequest(http.MethodGet, "/api/billing/admin/accounts"+scenario.query, nil)
			SearchBillingAccounts(ctx)
			require.Equal(t, scenario.status, recorder.Code)
			if scenario.status != 200 {
				return
			}
			var response struct {
				Success bool `json:"success"`
				Data    struct {
					Items []model.BillingAccountOption `json:"items"`
					Total int                          `json:"total"`
				} `json:"data"`
			}
			require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
			assert.True(t, response.Success)
			assert.Equal(t, scenario.total, response.Data.Total)
			ids := make([]int, 0, len(response.Data.Items))
			for _, item := range response.Data.Items {
				ids = append(ids, item.ID)
				if item.ID == 3 {
					assert.Equal(t, "Acme Ltd", item.CompanyTitle)
					assert.Equal(t, int64(100), item.AccountingStartAt)
				} else {
					assert.Zero(t, item.AccountingStartAt)
				}
			}
			assert.Equal(t, scenario.ids, ids)
			assert.NotContains(t, recorder.Body.String(), "private-password")
			assert.NotContains(t, recorder.Body.String(), "private-email")
		})
	}
}

func TestBillingAccountSwitchRequiresAdminAndFinancialCapability(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	saved := model.DB
	model.DB = db
	t.Cleanup(func() { model.DB = saved })
	setupBillingPermissions(t, db)
	require.NoError(t, authz.SetUserPermissions(2, authz.PermissionsMap{authz.ResourceFinancialAccounting: {authz.ActionFinancialView: false}}))
	require.NoError(t, authz.SetUserPermissions(4, authz.PermissionsMap{authz.ResourceFinancialAccounting: {authz.ActionFinancialView: true}}))
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.BillingAccount{}, &model.BillingStatement{}))
	require.NoError(t, db.Create(&model.BillingStatement{ID: "customer-document", UserID: 9, IssuedAt: 100}).Error)
	for _, scenario := range []struct {
		name        string
		actor, role int
		allowed     bool
	}{
		{"finance admin", 1, common.RoleAdminUser, true},
		{"admin with revoked finance", 2, common.RoleAdminUser, false},
		{"root", 3, common.RoleRootUser, true},
		{"non admin with explicit grant", 4, common.RoleCommonUser, false},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			for _, target := range []int{scenario.actor, 9} {
				recorder := httptest.NewRecorder()
				ctx, _ := gin.CreateTestContext(recorder)
				ctx.Set("id", scenario.actor)
				ctx.Set("role", scenario.role)
				ctx.Request = httptest.NewRequest(http.MethodGet, fmt.Sprintf("/account?user_id=%d", target), nil)
				id, ok := billingUserID(ctx)
				assert.Equal(t, target == scenario.actor || scenario.allowed, ok)
				if ok {
					assert.Equal(t, target, id)
				} else {
					assert.Equal(t, http.StatusForbidden, recorder.Code)
				}
			}
			recorder := httptest.NewRecorder()
			ctx, _ := gin.CreateTestContext(recorder)
			ctx.Set("id", scenario.actor)
			ctx.Set("role", scenario.role)
			ctx.Request = httptest.NewRequest(http.MethodGet, "/accounts", nil)
			SearchBillingAccounts(ctx)
			if scenario.allowed {
				assert.Equal(t, http.StatusOK, recorder.Code)
			} else {
				assert.Equal(t, http.StatusForbidden, recorder.Code)
			}
			recorder = httptest.NewRecorder()
			ctx, _ = gin.CreateTestContext(recorder)
			ctx.Set("id", scenario.actor)
			ctx.Set("role", scenario.role)
			ctx.Params = gin.Params{{Key: "statement_id", Value: "customer-document"}}
			_, ok := authorizedBillingStatement(ctx)
			assert.Equal(t, scenario.allowed, ok)
			if !ok {
				assert.Equal(t, http.StatusNotFound, recorder.Code)
			}
		})
	}
}
