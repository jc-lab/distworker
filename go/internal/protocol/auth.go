package protocol

import (
	"crypto/hmac"
	"crypto/sha256"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"
)

const (
	AuthPrefix       = "DISTWORKER1"
	SigningKeyData   = "distworker1_request"
	WSSigningKeyData = "distworker1_websocket"
	DateFormat       = "20060102T150405Z"
	DateOnlyFormat   = "20060102"
)

type ValidateContext struct {
	WorkerId          string
	SignedHeaders     string
	ProvidedSignature string
	Date              time.Time
	Headers           map[string]string
	HashedPayload     string

	Method string
	URL    *url.URL
}

// CanonicalRequest generates canonical request string for HMAC signing
func CanonicalRequest(method, uri, queryString string, headers map[string]string, signedHeaders []string, hashedPayload string) string {
	var canonicalHeaders strings.Builder

	// Sort headers for consistency
	sort.Strings(signedHeaders)

	for _, header := range signedHeaders {
		if value, exists := headers[strings.ToLower(header)]; exists {
			canonicalHeaders.WriteString(fmt.Sprintf("%s:%s\n", strings.ToLower(header), strings.TrimSpace(value)))
		}
	}

	signedHeadersStr := strings.Join(signedHeaders, ";")

	return fmt.Sprintf("%s\n%s\n%s\n%s\n%s\n%s",
		strings.ToUpper(method),
		uri,
		queryString,
		canonicalHeaders.String(),
		signedHeadersStr,
		hashedPayload,
	)
}

// GenerateSignature generates HMAC-SHA256 signature
func GenerateSignature(workerToken, date, canonicalRequest string) string {
	// DateKey = HMAC-SHA256(key = "DISTWORKER1" + worker_token, date = "<YYYYMMDD>")
	dateKey := hmacSHA256([]byte(AuthPrefix+workerToken), []byte(date))

	// SigningKey = HMAC-SHA256(key = <DateKey>, data = "distworker1_request")
	signingKey := hmacSHA256(dateKey, []byte(SigningKeyData))

	// Signature = HEX(HMAC-SHA256(key = <SigningKey>, data = <CanonicalRequest>))
	signature := hmacSHA256(signingKey, []byte(canonicalRequest))

	return fmt.Sprintf("%x", signature)
}

// GenerateWebSocketSignature generates HMAC-SHA256 signature for WebSocket
func GenerateWebSocketSignature(workerToken, date string, data []byte) []byte {
	// DateKey = HMAC-SHA256(key = "DISTWORKER1" + worker_token, date = "<YYYYMMDD>")
	dateKey := hmacSHA256([]byte(AuthPrefix+workerToken), []byte(date))

	// SigningKey = HMAC-SHA256(key = <DateKey>, data = "distworker1_websocket")
	signingKey := hmacSHA256(dateKey, []byte(WSSigningKeyData))

	// Signature = HMAC-SHA256(key = <SigningKey>, data = data)
	return hmacSHA256(signingKey, data)
}

func NewValidateContext(r *http.Request) (*ValidateContext, error) {
	vctx := &ValidateContext{
		Method: r.Method,
		URL:    r.URL,
	}

	// Extract Authorization header
	authHeader := r.Header.Get("Authorization")
	if !strings.HasPrefix(authHeader, "DISTWORKER1_HMAC_SHA256") {
		return vctx, fmt.Errorf("invalid authorization header format")
	}

	// Parse authorization header
	authParts := strings.Split(strings.TrimPrefix(authHeader, "DISTWORKER1_HMAC_SHA256 "), ", ")
	authMap := make(map[string]string)

	for _, part := range authParts {
		kv := strings.SplitN(part, "=", 2)
		if len(kv) == 2 {
			authMap[kv[0]] = kv[1]
		}
	}

	var exists bool

	vctx.WorkerId, exists = authMap["WorkerId"]
	if !exists {
		return vctx, fmt.Errorf("missing WorkerId in authorization header")
	}

	vctx.SignedHeaders, exists = authMap["SignedHeaders"]
	if !exists {
		return vctx, fmt.Errorf("missing SignedHeaders in authorization header")
	}

	vctx.ProvidedSignature, exists = authMap["Signature"]
	if !exists {
		return vctx, fmt.Errorf("missing Signature in authorization header")
	}

	// Get required headers
	dateHeader := r.Header.Get("x-distworker-date")
	if dateHeader == "" {
		return vctx, fmt.Errorf("missing x-distworker-date header")
	}

	// Validate date format and timing
	parsedTime, err := time.Parse(DateFormat, dateHeader)
	if err != nil {
		return vctx, fmt.Errorf("invalid date format: %v", err)
	}
	vctx.Date = parsedTime

	// Build headers map
	headers := make(map[string]string)
	vctx.HashedPayload = "UNSIGNED-PAYLOAD" // For now, we don't hash the payload
	for key, values := range r.Header {
		if len(values) > 0 {
			lowerKey := strings.ToLower(key)
			headers[lowerKey] = values[0]
			if lowerKey == "x-distworker-content-sha256" {
				vctx.HashedPayload = values[0]
			}
		}
	}
	vctx.Headers = headers

	// Check if request is within 5 minutes
	now := time.Now().UTC()
	if now.Sub(parsedTime).Abs() > 5*time.Minute {
		return vctx, fmt.Errorf("request timestamp too old or too far in future")
	}

	return vctx, nil
}

// ValidateSignature validates HTTP request signature
func (vctx *ValidateContext) ValidateSignature(workerToken string) error {
	// Generate canonical request
	canonicalReq := CanonicalRequest(
		vctx.Method,
		vctx.URL.Path,
		vctx.URL.RawQuery,
		vctx.Headers,
		strings.Split(vctx.SignedHeaders, ";"),
		vctx.HashedPayload,
	)

	// Generate expected signature
	date := vctx.Date.Format(DateOnlyFormat)
	expectedSignature := GenerateSignature(workerToken, date, canonicalReq)

	// Compare signatures
	if !hmac.Equal([]byte(vctx.ProvidedSignature), []byte(expectedSignature)) {
		return fmt.Errorf("signature mismatch")
	}

	return nil
}

// ValidateWebSocketSignature validates WebSocket message signature
func ValidateWebSocketSignature(workerToken, date string, data, signature []byte) error {
	// Parse date
	parsedTime, err := time.Parse(DateFormat, date)
	if err != nil {
		return fmt.Errorf("invalid date format: %v", err)
	}

	// Check if request is within 5 minutes
	now := time.Now().UTC()
	if now.Sub(parsedTime).Abs() > 5*time.Minute {
		return fmt.Errorf("request timestamp too old or too far in future")
	}

	// Generate expected signature
	dateOnly := parsedTime.Format(DateOnlyFormat)
	expectedSignature := GenerateWebSocketSignature(workerToken, dateOnly, data)

	// Compare signatures
	if !hmac.Equal(signature, expectedSignature) {
		return fmt.Errorf("signature mismatch")
	}

	return nil
}

// BuildAuthorizationHeader builds authorization header for HTTP requests
func BuildAuthorizationHeader(workerID, workerToken string, r *http.Request) string {
	// Set date header
	now := time.Now().UTC()
	dateStr := now.Format(DateFormat)
	r.Header.Set("x-distworker-date", dateStr)

	var signedHeaders []string

	// Build headers map
	hashedPayload := "UNSIGNED-PAYLOAD"
	headers := make(map[string]string)
	for key, values := range r.Header {
		lowerKey := strings.ToLower(key)
		if strings.HasPrefix(lowerKey, "x-distworker-") && len(values) > 0 {
			headers[lowerKey] = values[0]
			if key == "x-distworker-content-sha256" {
				hashedPayload = values[0]
			}
			signedHeaders = append(signedHeaders, lowerKey)
		}
	}

	// Generate canonical request
	canonicalReq := CanonicalRequest(
		r.Method,
		r.URL.Path,
		r.URL.RawQuery,
		headers,
		signedHeaders,
		hashedPayload,
	)

	// Generate signature
	date := now.Format(DateOnlyFormat)
	signature := GenerateSignature(workerToken, date, canonicalReq)

	return fmt.Sprintf("DISTWORKER1_HMAC_SHA256 WorkerId=%s, SignedHeaders=%s, Signature=%s",
		workerID, strings.Join(signedHeaders, ";"), signature)
}

// hmacSHA256 computes HMAC-SHA256
func hmacSHA256(key, data []byte) []byte {
	h := hmac.New(sha256.New, key)
	h.Write(data)
	return h.Sum(nil)
}
