package controller

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestBillingAccountSearchReturnsFormalIdentityAndSupportsCompanyTitle(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	savedDB := model.DB
	model.DB = db
	t.Cleanup(func() { model.DB = savedDB })
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.BillingAccount{}))
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
