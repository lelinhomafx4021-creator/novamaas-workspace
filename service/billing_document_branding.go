package service

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	"image/color"
	_ "image/gif"
	_ "image/jpeg"
	"image/png"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/system_setting"
	"golang.org/x/image/draw"
	_ "golang.org/x/image/webp"
	"golang.org/x/net/html"
)

type billingDocumentBranding struct {
	Issuer           string
	LogoPNG          []byte
	OperatingName    string
	OperatingLogoPNG []byte
	Footer           string
}

var ErrBillingDocumentBranding = errors.New("billing document branding is invalid")
var ErrBillingLogoFetchBlocked = errors.New("billing logo fetch blocked by the configured security policy")
var ErrBillingOperatingLogoUnconfigured = errors.New("billing operating entity logo is not configured")

// Branding is fetched once, before taking an accounting lock. Freeze the actual
// pixels and visible footer, not a mutable (potentially signed) URL, so retries
// and a later confirmation receipt never need to fetch current system settings.
func captureBillingDocumentBranding(ctx context.Context) (*billingDocumentBranding, error) {
	common.OptionMapRWMutex.RLock()
	issuer, logoURL, footer := common.SystemName, strings.TrimSpace(common.Logo), common.Footer
	operatingName, operatingLogoURL := strings.TrimSpace(common.OperatingEntityName), strings.TrimSpace(common.OperatingEntityLogo)
	serverAddress := system_setting.ServerAddress
	common.OptionMapRWMutex.RUnlock()
	plainFooter, err := billingFooterText(footer)
	if err != nil {
		return nil, err
	}
	logo, err := fetchBillingDocumentLogo(ctx, logoURL, billingPlatformMarkV2, "Logo", 24*1024, serverAddress, net.DefaultResolver)
	if err != nil {
		return nil, err
	}
	var operatingLogo []byte
	if operatingLogoURL != "" {
		operatingLogo, err = fetchBillingDocumentLogo(ctx, operatingLogoURL, nil, "OperatingEntityLogo", 12*1024, serverAddress, net.DefaultResolver)
		if err != nil {
			return nil, err
		}
		operatingLogo, err = normalizeBillingOperatingLogoWithin(operatingLogo, 12*1024)
		if err != nil {
			return nil, err
		}
	}
	return &billingDocumentBranding{Issuer: issuer, LogoPNG: logo, OperatingName: operatingName, OperatingLogoPNG: operatingLogo, Footer: plainFooter}, nil
}

// Test reports use the same trimmed operating mark as new billing documents.
// Fetch the administrator-configured image through the backend so browser
// canvas/CORS rules cannot leave square artwork with large blank margins.
func FetchPDFOperatingLogo(ctx context.Context) ([]byte, error) {
	common.OptionMapRWMutex.RLock()
	logoURL := strings.TrimSpace(common.OperatingEntityLogo)
	serverAddress := system_setting.ServerAddress
	common.OptionMapRWMutex.RUnlock()
	if logoURL == "" {
		return nil, ErrBillingOperatingLogoUnconfigured
	}
	logo, err := fetchBillingDocumentLogo(ctx, logoURL, nil, "OperatingEntityLogo", 12*1024, serverAddress, net.DefaultResolver)
	if err != nil {
		return nil, err
	}
	return normalizeBillingOperatingLogoWithin(logo, 12*1024)
}

func fetchBillingDocumentLogo(ctx context.Context, logoURL string, fallback []byte, label string, maxEncodedBytes int, serverAddress string, resolver ssrfResolver) ([]byte, error) {
	logo := fallback
	if logoURL != "" {
		parsed, err := url.Parse(logoURL)
		if err != nil {
			return nil, fmt.Errorf("invalid billing %s URL", label)
		}
		if !parsed.IsAbs() && strings.HasPrefix(logoURL, "/") {
			base, err := url.Parse(serverAddress)
			if err != nil {
				return nil, fmt.Errorf("invalid ServerAddress for billing %s URL", label)
			}
			parsed = base.ResolveReference(parsed)
		}
		if (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Hostname() == "" || parsed.User != nil {
			return nil, fmt.Errorf("billing %s URL must be an HTTP(S) image without credentials", label)
		}
		client := GetSSRFProtectedHTTPClient()
		if billingLogoUsesFakeIP(ctx, parsed, resolver) {
			// The operating system's fake-IP DNS maps this address back to the
			// administrator-configured domain. The general protected dialer must
			// continue rejecting this range for user-controlled fetches.
			baseClient := GetHttpClient()
			if baseClient == nil {
				return nil, fmt.Errorf("billing %s HTTP client is not initialized", label)
			}
			scopedClient := *baseClient
			scopedClient.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
			client = &scopedClient
		} else if err := ValidateSSRFProtectedFetchURL(parsed.String()); err != nil {
			return nil, fmt.Errorf("%w: %s URL: %v", ErrBillingLogoFetchBlocked, label, err)
		}
		ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()
		request, err := http.NewRequestWithContext(ctx, http.MethodGet, parsed.String(), nil)
		if err != nil {
			return nil, fmt.Errorf("invalid billing %s request", label)
		}
		request.Header.Set("Accept", "image/png,image/jpeg,image/webp,image/gif")
		if client == nil {
			return nil, fmt.Errorf("billing %s HTTP client is not initialized", label)
		}
		response, err := client.Do(request)
		if err != nil {
			return nil, fmt.Errorf("billing %s download failed; check the configured URL and network access", label)
		}
		defer response.Body.Close()
		if response.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("billing %s download returned HTTP %d", label, response.StatusCode)
		}
		const maxDownload = 2 * 1024 * 1024
		if response.ContentLength > maxDownload {
			return nil, fmt.Errorf("billing %s exceeds 2 MiB", label)
		}
		logo, err = io.ReadAll(io.LimitReader(response.Body, maxDownload+1))
		if err != nil || len(logo) > maxDownload {
			return nil, fmt.Errorf("billing %s could not be read within the 2 MiB limit", label)
		}
	}
	return normalizeBillingLogoWithin(logo, maxEncodedBytes)
}

// A fake-IP is usable only when every DNS answer for an administrator-owned
// HTTPS logo domain belongs to the TUN range and the configured domain, port,
// and explicit IP list permit it. Literal IP URLs never get this exception.
func billingLogoUsesFakeIP(ctx context.Context, parsed *url.URL, resolver ssrfResolver) bool {
	if parsed.Scheme != "https" || net.ParseIP(parsed.Hostname()) != nil || resolver == nil {
		return false
	}
	protection, enabled, err := currentFetchProtection()
	if err != nil || !enabled || !protection.ApplyIPFilterForDomain {
		return false
	}
	port := 443
	if parsed.Port() != "" {
		port, err = strconv.Atoi(parsed.Port())
		if err != nil {
			return false
		}
	}
	if err := protection.ValidateNetworkTarget(parsed.Hostname(), port); err != nil {
		return false
	}
	resolved, err := resolver.LookupIPAddr(ctx, parsed.Hostname())
	if err != nil || len(resolved) == 0 {
		return false
	}
	for _, address := range resolved {
		ip := address.IP.To4()
		if ip == nil || ip[0] != 198 || (ip[1] != 18 && ip[1] != 19) {
			return false
		}
		listed := common.IsIpInCIDRList(ip, protection.IpList)
		if protection.IpFilterMode != listed {
			return false
		}
	}
	return true
}

// Rasterize to a bounded PNG without distorting the aspect ratio or fetching
// external image resources. The limit also keeps the full JSON snapshot within
// MySQL's existing TEXT column, including base64 overhead and all 31 days.
func normalizeBillingLogo(body []byte) ([]byte, error) {
	return normalizeBillingLogoWithin(body, 24*1024)
}

// Operating entity marks are often horizontal artwork on a square white or
// transparent canvas. Trim only the outer blank area before fitting the PDF.
func normalizeBillingOperatingLogoWithin(body []byte, maxEncodedBytes int) ([]byte, error) {
	source, _, err := image.Decode(bytes.NewReader(body))
	if err != nil {
		return nil, errors.New("billing OperatingEntityLogo must be a valid image")
	}
	bounds := source.Bounds()
	minX, minY, maxX, maxY := bounds.Max.X, bounds.Max.Y, bounds.Min.X, bounds.Min.Y
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			pixel := color.NRGBAModel.Convert(source.At(x, y)).(color.NRGBA)
			if pixel.A <= 16 || (pixel.R >= 247 && pixel.G >= 247 && pixel.B >= 247) {
				continue
			}
			minX, minY = min(minX, x), min(minY, y)
			maxX, maxY = max(maxX, x+1), max(maxY, y+1)
		}
	}
	if minX >= maxX || minY >= maxY {
		return normalizeBillingLogoWithin(body, maxEncodedBytes)
	}
	padding := max(2, min(bounds.Dx(), bounds.Dy())/32)
	crop := image.Rect(max(bounds.Min.X, minX-padding), max(bounds.Min.Y, minY-padding), min(bounds.Max.X, maxX+padding), min(bounds.Max.Y, maxY+padding))
	if crop == bounds {
		return normalizeBillingLogoWithin(body, maxEncodedBytes)
	}
	trimmed := image.NewNRGBA(image.Rect(0, 0, crop.Dx(), crop.Dy()))
	draw.Draw(trimmed, trimmed.Bounds(), source, crop.Min, draw.Src)
	var encoded bytes.Buffer
	if err := png.Encode(&encoded, trimmed); err != nil {
		return nil, err
	}
	return normalizeBillingLogoWithin(encoded.Bytes(), maxEncodedBytes)
}

func normalizeBillingLogoWithin(body []byte, maxEncodedBytes int) ([]byte, error) {
	config, _, err := image.DecodeConfig(bytes.NewReader(body))
	if err != nil || config.Width <= 0 || config.Height <= 0 {
		return nil, errors.New("billing Logo must be a valid PNG, JPEG, GIF or WebP image")
	}
	if config.Width > 4096 || config.Height > 4096 || int64(config.Width)*int64(config.Height) > 16*1024*1024 {
		return nil, errors.New("billing Logo dimensions exceed 4096 pixels or 16 megapixels")
	}
	source, _, err := image.Decode(bytes.NewReader(body))
	if err != nil {
		return nil, errors.New("billing Logo image is damaged")
	}
	// Prefer 256 pixels for the wide operating mark and the 38-point platform
	// mark, reducing unusually complex images only to keep the archive compact.
	for _, maxSide := range []int{256, 128, 96, 64, 48, 32} {
		width, height := config.Width, config.Height
		if longest := max(width, height); longest > maxSide {
			width = max(1, width*maxSide/longest)
			height = max(1, height*maxSide/longest)
		}
		normalized := image.NewNRGBA(image.Rect(0, 0, width, height))
		draw.CatmullRom.Scale(normalized, normalized.Bounds(), source, source.Bounds(), draw.Src, nil)
		var encoded bytes.Buffer
		encoder := png.Encoder{CompressionLevel: png.BestCompression}
		if err := encoder.Encode(&encoded, normalized); err != nil {
			return nil, err
		}
		if encoded.Len() <= maxEncodedBytes {
			return encoded.Bytes(), nil
		}
	}
	return nil, errors.New("billing Logo cannot fit the document image limit")
}

// Site footers can contain HTML. Archive only visible text, never execute HTML
// or fetch a footer's scripts/images while preparing a financial document.
func billingFooterText(footer string) (string, error) {
	if len(footer) > 64*1024 {
		return "", errors.New("billing Footer HTML exceeds 64 KiB")
	}
	tokenizer := html.NewTokenizer(strings.NewReader(footer))
	var text strings.Builder
	hidden := ""
	for {
		tokenType := tokenizer.Next()
		if tokenType == html.ErrorToken {
			if err := tokenizer.Err(); err != io.EOF {
				return "", err
			}
			break
		}
		token := tokenizer.Token()
		if hidden != "" {
			if tokenType == html.EndTagToken && token.Data == hidden {
				hidden = ""
			}
			continue
		}
		if tokenType == html.StartTagToken {
			switch token.Data {
			case "script", "style", "iframe", "noscript", "template":
				hidden = token.Data
				continue
			}
		}
		if tokenType == html.TextToken {
			text.WriteString(token.Data)
		} else if tokenType == html.StartTagToken || tokenType == html.EndTagToken || tokenType == html.SelfClosingTagToken {
			switch token.Data {
			case "br", "p", "div", "li", "hr", "section", "footer":
				text.WriteByte(' ')
			}
		}
	}
	plain := strings.Join(strings.FieldsFunc(text.String(), func(r rune) bool { return unicode.IsSpace(r) || unicode.IsControl(r) }), " ")
	if len([]rune(plain)) > 300 {
		return "", errors.New("billing Footer exceeds 300 visible characters; shorten the system Footer")
	}
	return plain, nil
}

// Customer download names are presentation only. Internal UUIDs, object keys
// and evidence digests remain unchanged; downloading again keeps the same name.
// Use the username frozen in the statement snapshot so later account renames do
// not change the name of already archived evidence.
func BillingArtifactFilename(statement *model.BillingStatement, username, kind string, ordinal int) string {
	username = strings.TrimSpace(username)
	username = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) || strings.ContainsRune(`/\:*?"<>|`, r) {
			return '_'
		}
		return r
	}, username)
	username = strings.Trim(username, ". ")
	if username == "" {
		username = fmt.Sprintf("user-%d", statement.UserID)
	}
	if runes := []rune(username); len(runes) > 64 {
		username = string(runes[:64])
	}
	if kind == "pdf" {
		return fmt.Sprintf("%s_%s_月度对账单_V%02d.pdf", username, statement.Month, statement.Revision)
	}
	if kind == "receipt" {
		return fmt.Sprintf("%s_%s_对账确认回执_V%02d.pdf", username, statement.Month, statement.Revision)
	}
	extension := "json"
	if kind == "details" {
		extension = "jsonl.gz"
	}
	return fmt.Sprintf("statement-%s-r%d-%s-%d.%s", statement.Month, statement.Revision, kind, ordinal, extension)
}
