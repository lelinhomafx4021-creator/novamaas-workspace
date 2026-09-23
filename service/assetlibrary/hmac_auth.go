package assetlibrary

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"

	"gorm.io/gorm"
)

const volcActionRequestClockSkew = 5 * time.Minute

type volcAuthorization struct {
	AccessKeyID   string
	ShortDate     string
	Region        string
	Service       string
	SignedHeaders string
	Signature     string
}

type VolcActionPrincipal struct {
	OwnerUserID   int
	AccessKeyID   string
	AccessKeyName string
}

func AuthenticateVolcActionRequest(request *http.Request, body []byte) (int, error) {
	principal, err := AuthenticateVolcActionRequestPrincipal(request, body)
	if err != nil {
		return 0, err
	}
	return principal.OwnerUserID, nil
}

func authenticateVolcActionRequestAt(request *http.Request, body []byte, now time.Time) (int, error) {
	principal, err := authenticateVolcActionRequestPrincipalAt(request, body, now)
	if err != nil {
		return 0, err
	}
	return principal.OwnerUserID, nil
}

func AuthenticateVolcActionRequestPrincipal(request *http.Request, body []byte) (*VolcActionPrincipal, error) {
	return authenticateVolcActionRequestPrincipalAt(request, body, time.Now().UTC())
}

func authenticateVolcActionRequestPrincipalAt(request *http.Request, body []byte, now time.Time) (*VolcActionPrincipal, error) {
	if request == nil || request.URL == nil {
		return nil, errors.New("invalid signed request")
	}
	auth, err := parseVolcAuthorization(request.Header.Get("Authorization"))
	if err != nil {
		return nil, err
	}
	if auth.Region != DefaultRegion || auth.Service != DefaultService {
		return nil, errors.New("credential scope must use cn-beijing/ark")
	}
	xDate := strings.TrimSpace(request.Header.Get("X-Date"))
	requestTime, err := time.Parse("20060102T150405Z", xDate)
	if err != nil || auth.ShortDate != requestTime.UTC().Format("20060102") {
		return nil, errors.New("invalid X-Date or credential date")
	}
	clockDifference := now.Sub(requestTime)
	if clockDifference < -volcActionRequestClockSkew || clockDifference > volcActionRequestClockSkew {
		return nil, errors.New("signed request time is outside the allowed window")
	}
	payloadDigest := sha256.Sum256(body)
	payloadHash := hex.EncodeToString(payloadDigest[:])
	if !strings.EqualFold(payloadHash, strings.TrimSpace(request.Header.Get("X-Content-Sha256"))) {
		return nil, errors.New("X-Content-Sha256 does not match the request body")
	}

	var key model.AssetAccessKey
	if err = model.DB.Where("access_key_id = ? AND status = ?", auth.AccessKeyID, model.AssetAccessKeyStatusEnabled).First(&key).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("invalid access key")
		}
		return nil, err
	}
	secret, err := decryptAccessKeySecret(key.EncryptedSecret)
	if err != nil {
		return nil, fmt.Errorf("decrypt asset access key: %w", err)
	}

	host := strings.TrimSpace(request.Host)
	if host == "" {
		host = request.URL.Host
	}
	canonicalHeaders := ""
	switch auth.SignedHeaders {
	case "content-type;host;x-content-sha256;x-date":
		canonicalHeaders = "content-type:" + strings.TrimSpace(request.Header.Get("Content-Type")) + "\n"
	case "host;x-content-sha256;x-date":
	default:
		return nil, errors.New("unsupported signed headers")
	}
	canonicalHeaders += "host:" + host + "\n" +
		"x-content-sha256:" + payloadHash + "\n" +
		"x-date:" + xDate + "\n"
	canonicalRequest := strings.Join([]string{
		request.Method,
		canonicalURI(request.URL),
		canonicalQuery(request.URL.Query()),
		canonicalHeaders,
		auth.SignedHeaders,
		payloadHash,
	}, "\n")
	canonicalHash := sha256.Sum256([]byte(canonicalRequest))
	scope := strings.Join([]string{auth.ShortDate, auth.Region, auth.Service, "request"}, "/")
	stringToSign := "HMAC-SHA256\n" + xDate + "\n" + scope + "\n" + hex.EncodeToString(canonicalHash[:])
	kDate := hmacSHA256([]byte(secret), []byte(auth.ShortDate))
	kRegion := hmacSHA256(kDate, []byte(auth.Region))
	kService := hmacSHA256(kRegion, []byte(auth.Service))
	kSigning := hmacSHA256(kService, []byte("request"))
	expectedSignature := hmacSHA256(kSigning, []byte(stringToSign))
	providedSignature, err := hex.DecodeString(auth.Signature)
	if err != nil || !hmac.Equal(expectedSignature, providedSignature) {
		return nil, errors.New("invalid request signature")
	}

	user, err := model.GetUserCache(key.OwnerUserID)
	if err != nil {
		return nil, err
	}
	if user.Status != common.UserStatusEnabled {
		return nil, errors.New("asset access key owner is unavailable")
	}
	usedAt := now.Unix()
	_ = model.DB.Model(&model.AssetAccessKey{}).
		Where("id = ? AND last_used_at < ?", key.ID, usedAt-60).
		Updates(map[string]any{"last_used_at": usedAt, "updated_at": usedAt}).Error
	return &VolcActionPrincipal{
		OwnerUserID:   key.OwnerUserID,
		AccessKeyID:   key.AccessKeyID,
		AccessKeyName: key.Name,
	}, nil
}

func parseVolcAuthorization(value string) (*volcAuthorization, error) {
	value = strings.TrimSpace(value)
	if !strings.HasPrefix(value, "HMAC-SHA256 ") {
		return nil, errors.New("HMAC-SHA256 authorization is required")
	}
	attributes := make(map[string]string, 3)
	for _, part := range strings.Split(strings.TrimPrefix(value, "HMAC-SHA256 "), ",") {
		key, fieldValue, ok := strings.Cut(strings.TrimSpace(part), "=")
		if !ok || fieldValue == "" {
			return nil, errors.New("invalid HMAC-SHA256 authorization")
		}
		attributes[key] = fieldValue
	}
	credential := strings.Split(attributes["Credential"], "/")
	if len(credential) != 5 || credential[4] != "request" {
		return nil, errors.New("invalid credential scope")
	}
	if attributes["SignedHeaders"] == "" || attributes["Signature"] == "" {
		return nil, errors.New("signed headers and signature are required")
	}
	return &volcAuthorization{
		AccessKeyID: credential[0], ShortDate: credential[1], Region: credential[2], Service: credential[3],
		SignedHeaders: attributes["SignedHeaders"], Signature: attributes["Signature"],
	}, nil
}
