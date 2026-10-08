package controller

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestBillingCorrectionRequiresRootSessionScopedProofAndExactConfirmation(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	savedDB, savedSecret := model.DB, common.SessionSecret
	model.DB, common.SessionSecret = db, "billing-correction-controller-test"
	t.Cleanup(func() { model.DB, common.SessionSecret = savedDB, savedSecret })
	require.NoError(t, db.AutoMigrate(&model.BillingCorrection{}, &model.BillingCorrectionRow{}))
	require.NoError(t, db.Create(&model.BillingCorrection{ID: "batch", UserID: 4, NetDelta: 70715}).Error)
	identity := service.AuthIdentity{UserID: 1, SessionID: "live", UserAuthVersion: 1, SessionVersion: 1}
	proof, _, err := service.IssueSecurityProof(identity, "2fa", []string{"billing.correct"})
	require.NoError(t, err)
	wrongScope, _, err := service.IssueSecurityProof(identity, "2fa", []string{"channel.key.read"})
	require.NoError(t, err)
	for _, test := range []struct {
		name, path, session, proof, body string
		role, status                     int
	}{
		{"admin cannot preview", "/preview", "live", "", "{}", common.RoleAdminUser, 403},
		{"PAT cannot preview", "/preview", "", "", "{}", common.RoleRootUser, 403},
		{"customer cannot execute", "/batch/actions", "live", proof, "{}", common.RoleCommonUser, 403},
		{"PAT cannot execute", "/batch/actions", "", proof, "{}", common.RoleRootUser, 403},
		{"missing proof", "/batch/actions", "live", "", "{}", common.RoleRootUser, 403},
		{"proof from another session", "/batch/actions", "other", proof, "{}", common.RoleRootUser, 403},
		{"wrong proof scope", "/batch/actions", "live", wrongScope, "{}", common.RoleRootUser, 403},
		{"wrong customer confirmation", "/batch/actions", "live", proof, `{"action":"apply","confirm_user_id":5,"confirm_net_delta":70715}`, common.RoleRootUser, 409},
		{"wrong amount confirmation", "/batch/actions", "live", proof, `{"action":"apply","confirm_user_id":4,"confirm_net_delta":1}`, common.RoleRootUser, 409},
		{"missing amount confirmation", "/batch/actions", "live", proof, `{"action":"apply","confirm_user_id":4}`, common.RoleRootUser, 409},
	} {
		t.Run(test.name, func(t *testing.T) {
			router := gin.New()
			router.Use(func(c *gin.Context) {
				c.Set("id", 1)
				c.Set("role", test.role)
				c.Set("session_id", test.session)
				c.Set("auth_version", int64(1))
				c.Set("session_version", int64(1))
			})
			router.POST("/preview", PreviewBillingCorrection)
			router.POST("/:correction_id/actions", ActOnBillingCorrection)
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodPost, test.path, strings.NewReader(test.body))
			request.Header.Set("X-Security-Proof", test.proof)
			router.ServeHTTP(recorder, request)
			assert.Equal(t, test.status, recorder.Code, recorder.Body.String())
		})
	}
}
