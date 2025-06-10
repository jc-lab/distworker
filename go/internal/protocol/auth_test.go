// distworker
// Copyright (C) 2025 JC-Lab
//
// SPDX-License-Identifier: AGPL-3.0-only
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package protocol

import (
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestCanonicalRequest(t *testing.T) {
	t.Run("BasicCanonicalRequest", func(t *testing.T) {
		method := "GET"
		uri := "/api/v1/tasks"
		queryString := "status=pending&limit=10"
		headers := map[string]string{
			"x-distworker-date": "20220830T123600Z",
			"host":              "localhost:8080",
		}
		signedHeaders := []string{"x-distworker-date", "host"}
		hashedPayload := "UNSIGNED-PAYLOAD"

		canonical := CanonicalRequest(method, uri, queryString, headers, signedHeaders, hashedPayload)

		expected := "GET\n" +
			"/api/v1/tasks\n" +
			"status=pending&limit=10\n" +
			"host:localhost:8080\n" +
			"x-distworker-date:20220830T123600Z\n" +
			"\n" +
			"host;x-distworker-date\n" +
			"UNSIGNED-PAYLOAD"

		if canonical != expected {
			t.Errorf("Canonical request mismatch.\nExpected:\n%s\n\nGot:\n%s", expected, canonical)
		}
	})

	t.Run("EmptyQueryString", func(t *testing.T) {
		method := "POST"
		uri := "/api/v1/tasks"
		queryString := ""
		headers := map[string]string{
			"x-distworker-date": "20220830T123600Z",
		}
		signedHeaders := []string{"x-distworker-date"}
		hashedPayload := "UNSIGNED-PAYLOAD"

		canonical := CanonicalRequest(method, uri, queryString, headers, signedHeaders, hashedPayload)

		expected := "POST\n" +
			"/api/v1/tasks\n" +
			"\n" +
			"x-distworker-date:20220830T123600Z\n" +
			"\n" +
			"x-distworker-date\n" +
			"UNSIGNED-PAYLOAD"

		if canonical != expected {
			t.Errorf("Canonical request mismatch.\nExpected:\n%s\n\nGot:\n%s", expected, canonical)
		}
	})

	t.Run("HeaderSorting", func(t *testing.T) {
		method := "GET"
		uri := "/api/v1/test"
		queryString := ""
		headers := map[string]string{
			"z-header":          "value-z",
			"a-header":          "value-a",
			"x-distworker-date": "20220830T123600Z",
			"m-header":          "value-m",
		}
		signedHeaders := []string{"z-header", "a-header", "x-distworker-date", "m-header"}
		hashedPayload := "UNSIGNED-PAYLOAD"

		canonical := CanonicalRequest(method, uri, queryString, headers, signedHeaders, hashedPayload)

		// Headers should be sorted in canonical order
		expectedHeadersSection := "a-header:value-a\n" +
			"m-header:value-m\n" +
			"x-distworker-date:20220830T123600Z\n" +
			"z-header:value-z\n"

		if !strings.Contains(canonical, expectedHeadersSection) {
			t.Errorf("Headers not properly sorted in canonical request.\nCanonical:\n%s", canonical)
		}

		// Signed headers should also be sorted
		expectedSignedHeaders := "a-header;m-header;x-distworker-date;z-header"
		if !strings.Contains(canonical, expectedSignedHeaders) {
			t.Errorf("Signed headers not properly sorted.\nCanonical:\n%s", canonical)
		}
	})

	t.Run("HeaderTrimming", func(t *testing.T) {
		method := "GET"
		uri := "/test"
		queryString := ""
		headers := map[string]string{
			"x-distworker-date": "  20220830T123600Z  ", // Extra spaces
		}
		signedHeaders := []string{"x-distworker-date"}
		hashedPayload := "UNSIGNED-PAYLOAD"

		canonical := CanonicalRequest(method, uri, queryString, headers, signedHeaders, hashedPayload)

		// Should trim spaces from header values
		expectedHeaderLine := "x-distworker-date:20220830T123600Z\n"
		if !strings.Contains(canonical, expectedHeaderLine) {
			t.Errorf("Header value not properly trimmed.\nCanonical:\n%s", canonical)
		}
	})
}

func TestGenerateSignature(t *testing.T) {
	t.Run("BasicSignature", func(t *testing.T) {
		workerToken := "test-token-123"
		date := "20220830"
		canonicalRequest := "GET\n/api/v1/test\n\nx-distworker-date:20220830T123600Z\nx-distworker-date\nUNSIGNED-PAYLOAD"

		signature := GenerateSignature(workerToken, date, canonicalRequest)

		// Signature should be a hex string
		if len(signature) == 0 {
			t.Error("Expected non-empty signature")
		}

		// Should be consistent
		signature2 := GenerateSignature(workerToken, date, canonicalRequest)
		if signature != signature2 {
			t.Error("Signature should be deterministic")
		}
	})

	t.Run("DifferentInputsDifferentSignatures", func(t *testing.T) {
		workerToken := "test-token-123"
		date := "20220830"
		canonicalRequest1 := "GET\n/api/v1/test1\n\nx-distworker-date:20220830T123600Z\nx-distworker-date\nUNSIGNED-PAYLOAD"
		canonicalRequest2 := "GET\n/api/v1/test2\n\nx-distworker-date:20220830T123600Z\nx-distworker-date\nUNSIGNED-PAYLOAD"

		signature1 := GenerateSignature(workerToken, date, canonicalRequest1)
		signature2 := GenerateSignature(workerToken, date, canonicalRequest2)

		if signature1 == signature2 {
			t.Error("Different canonical requests should produce different signatures")
		}
	})

	t.Run("DifferentTokensDifferentSignatures", func(t *testing.T) {
		date := "20220830"
		canonicalRequest := "GET\n/api/v1/test\n\nx-distworker-date:20220830T123600Z\nx-distworker-date\nUNSIGNED-PAYLOAD"

		signature1 := GenerateSignature("token1", date, canonicalRequest)
		signature2 := GenerateSignature("token2", date, canonicalRequest)

		if signature1 == signature2 {
			t.Error("Different worker tokens should produce different signatures")
		}
	})

	t.Run("DifferentDatesDifferentSignatures", func(t *testing.T) {
		workerToken := "test-token-123"
		canonicalRequest := "GET\n/api/v1/test\n\nx-distworker-date:20220830T123600Z\nx-distworker-date\nUNSIGNED-PAYLOAD"

		signature1 := GenerateSignature(workerToken, "20220830", canonicalRequest)
		signature2 := GenerateSignature(workerToken, "20220831", canonicalRequest)

		if signature1 == signature2 {
			t.Error("Different dates should produce different signatures")
		}
	})
}

func TestGenerateWebSocketSignature(t *testing.T) {
	t.Run("BasicWebSocketSignature", func(t *testing.T) {
		workerToken := "test-token-123"
		date := "20220830"
		data := []byte("test websocket data")

		signature := GenerateWebSocketSignature(workerToken, date, data)

		if len(signature) == 0 {
			t.Error("Expected non-empty signature")
		}

		// Should be consistent
		signature2 := GenerateWebSocketSignature(workerToken, date, data)
		if string(signature) != string(signature2) {
			t.Error("WebSocket signature should be deterministic")
		}
	})

	t.Run("DifferentDataDifferentSignatures", func(t *testing.T) {
		workerToken := "test-token-123"
		date := "20220830"
		data1 := []byte("test data 1")
		data2 := []byte("test data 2")

		signature1 := GenerateWebSocketSignature(workerToken, date, data1)
		signature2 := GenerateWebSocketSignature(workerToken, date, data2)

		if string(signature1) == string(signature2) {
			t.Error("Different data should produce different signatures")
		}
	})
}

func TestValidateSignature(t *testing.T) {
	workerToken := "test-token-123"

	t.Run("ValidSignature", func(t *testing.T) {
		// Create a request
		req, err := http.NewRequest("GET", "http://localhost:8080/api/v1/test", nil)
		if err != nil {
			t.Fatalf("Failed to create request: %v", err)
		}

		// Set date header
		now := time.Now().UTC()
		dateStr := now.Format(DateFormat)
		req.Header.Set("x-distworker-date", dateStr)

		// Generate authorization header
		workerId := "test-worker-123"
		authHeader := BuildAuthorizationHeader(workerId, workerToken, req)
		req.Header.Set("Authorization", authHeader)

		// Validate signature
		vctx, err := validateSignature(workerToken, req)
		if err != nil {
			t.Errorf("Expected valid signature to pass validation, got: %v", err)
		}

		// Check that worker ID was set in headers
		if vctx.WorkerId != workerId {
			t.Errorf("Expected workerId to be set to %s, got: %s", workerId, vctx.WorkerId)
		}
	})

	t.Run("MissingAuthorizationHeader", func(t *testing.T) {
		req, err := http.NewRequest("GET", "http://localhost:8080/api/v1/test", nil)
		if err != nil {
			t.Fatalf("Failed to create request: %v", err)
		}

		_, err = validateSignature(workerToken, req)
		if err == nil {
			t.Error("Expected error for missing authorization header")
		}

		if !strings.Contains(err.Error(), "invalid authorization header format") {
			t.Errorf("Expected error about authorization header format, got: %v", err)
		}
	})

	t.Run("InvalidAuthorizationHeaderFormat", func(t *testing.T) {
		req, err := http.NewRequest("GET", "http://localhost:8080/api/v1/test", nil)
		if err != nil {
			t.Fatalf("Failed to create request: %v", err)
		}

		req.Header.Set("Authorization", "Bearer token123")

		_, err = validateSignature(workerToken, req)
		if err == nil {
			t.Error("Expected error for invalid authorization header format")
		}

		if !strings.Contains(err.Error(), "invalid authorization header format") {
			t.Errorf("Expected error about authorization header format, got: %v", err)
		}
	})

	t.Run("MissingDateHeader", func(t *testing.T) {
		req, err := http.NewRequest("GET", "http://localhost:8080/api/v1/test", nil)
		if err != nil {
			t.Fatalf("Failed to create request: %v", err)
		}

		req.Header.Set("Authorization", "DISTWORKER1_HMAC_SHA256 workerId=test, SignedHeaders=x-distworker-date, Signature=abc123")

		_, err = validateSignature(workerToken, req)
		if err == nil {
			t.Error("Expected error for missing date header")
		}

		if !strings.Contains(err.Error(), "missing x-distworker-date header") {
			t.Errorf("Expected error about missing date header, got: %v", err)
		}
	})

	t.Run("InvalidDateFormat", func(t *testing.T) {
		req, err := http.NewRequest("GET", "http://localhost:8080/api/v1/test", nil)
		if err != nil {
			t.Fatalf("Failed to create request: %v", err)
		}

		req.Header.Set("x-distworker-date", "2022-08-30 12:36:00") // Wrong format
		req.Header.Set("Authorization", "DISTWORKER1_HMAC_SHA256 workerId=test, SignedHeaders=x-distworker-date, Signature=abc123")

		_, err = validateSignature(workerToken, req)
		if err == nil {
			t.Error("Expected error for invalid date format")
		}

		if !strings.Contains(err.Error(), "invalid date format") {
			t.Errorf("Expected error about invalid date format, got: %v", err)
		}
	})

	t.Run("TimestampTooOld", func(t *testing.T) {
		req, err := http.NewRequest("GET", "http://localhost:8080/api/v1/test", nil)
		if err != nil {
			t.Fatalf("Failed to create request: %v", err)
		}

		// Set date to 10 minutes ago
		oldTime := time.Now().UTC().Add(-10 * time.Minute)
		req.Header.Set("x-distworker-date", oldTime.Format(DateFormat))
		req.Header.Set("Authorization", "DISTWORKER1_HMAC_SHA256 workerId=test, SignedHeaders=x-distworker-date, Signature=abc123")

		_, err = validateSignature(workerToken, req)
		if err == nil {
			t.Error("Expected error for timestamp too old")
		}

		if !strings.Contains(err.Error(), "timestamp too old") {
			t.Errorf("Expected error about timestamp being too old, got: %v", err)
		}
	})

	t.Run("TimestampTooFuture", func(t *testing.T) {
		req, err := http.NewRequest("GET", "http://localhost:8080/api/v1/test", nil)
		if err != nil {
			t.Fatalf("Failed to create request: %v", err)
		}

		// Set date to 10 minutes in the future
		futureTime := time.Now().UTC().Add(10 * time.Minute)
		req.Header.Set("x-distworker-date", futureTime.Format(DateFormat))
		req.Header.Set("Authorization", "DISTWORKER1_HMAC_SHA256 workerId=test, SignedHeaders=x-distworker-date, Signature=abc123")

		_, err = validateSignature(workerToken, req)
		if err == nil {
			t.Error("Expected error for timestamp too far in future")
		}

		if !strings.Contains(err.Error(), "too far in future") {
			t.Errorf("Expected error about timestamp being too far in future, got: %v", err)
		}
	})

	t.Run("InvalidSignature", func(t *testing.T) {
		req, err := http.NewRequest("GET", "http://localhost:8080/api/v1/test", nil)
		if err != nil {
			t.Fatalf("Failed to create request: %v", err)
		}

		// Set valid date but invalid signature
		now := time.Now().UTC()
		req.Header.Set("x-distworker-date", now.Format(DateFormat))
		req.Header.Set("Authorization", "DISTWORKER1_HMAC_SHA256 workerId=test, SignedHeaders=x-distworker-date, Signature=invalid-signature")

		_, err = validateSignature(workerToken, req)
		if err == nil {
			t.Error("Expected error for invalid signature")
		}

		if !strings.Contains(err.Error(), "signature mismatch") {
			t.Errorf("Expected error about signature mismatch, got: %v", err)
		}
	})

	t.Run("MissingWorkerIdInAuth", func(t *testing.T) {
		req, err := http.NewRequest("GET", "http://localhost:8080/api/v1/test", nil)
		if err != nil {
			t.Fatalf("Failed to create request: %v", err)
		}

		now := time.Now().UTC()
		req.Header.Set("x-distworker-date", now.Format(DateFormat))
		req.Header.Set("Authorization", "DISTWORKER1_HMAC_SHA256 SignedHeaders=x-distworker-date, Signature=abc123")

		_, err = validateSignature(workerToken, req)
		if err == nil {
			t.Error("Expected error for missing workerId")
		}

		if !strings.Contains(err.Error(), "missing workerId") {
			t.Errorf("Expected error about missing workerId, got: %v", err)
		}
	})
}

func TestValidateWebSocketSignature(t *testing.T) {
	workerToken := "test-token-123"

	t.Run("ValidWebSocketSignature", func(t *testing.T) {
		now := time.Now().UTC()
		dateStr := now.Format(DateFormat)
		data := []byte("test websocket message data")

		// Generate signature
		dateOnly := now.Format(DateOnlyFormat)
		signature := GenerateWebSocketSignature(workerToken, dateOnly, data)

		// Validate signature
		err := ValidateWebSocketSignature(workerToken, dateStr, data, signature)
		if err != nil {
			t.Errorf("Expected valid WebSocket signature to pass validation, got: %v", err)
		}
	})

	t.Run("InvalidWebSocketDateFormat", func(t *testing.T) {
		data := []byte("test data")
		signature := []byte("test signature")

		err := ValidateWebSocketSignature(workerToken, "invalid-date", data, signature)
		if err == nil {
			t.Error("Expected error for invalid date format")
		}

		if !strings.Contains(err.Error(), "invalid date format") {
			t.Errorf("Expected error about invalid date format, got: %v", err)
		}
	})

	t.Run("WebSocketTimestampTooOld", func(t *testing.T) {
		// Set date to 10 minutes ago
		oldTime := time.Now().UTC().Add(-10 * time.Minute)
		dateStr := oldTime.Format(DateFormat)
		data := []byte("test data")
		signature := []byte("test signature")

		err := ValidateWebSocketSignature(workerToken, dateStr, data, signature)
		if err == nil {
			t.Error("Expected error for timestamp too old")
		}

		if !strings.Contains(err.Error(), "timestamp too old") {
			t.Errorf("Expected error about timestamp being too old, got: %v", err)
		}
	})

	t.Run("WebSocketInvalidSignature", func(t *testing.T) {
		now := time.Now().UTC()
		dateStr := now.Format(DateFormat)
		data := []byte("test data")
		invalidSignature := []byte("invalid signature")

		err := ValidateWebSocketSignature(workerToken, dateStr, data, invalidSignature)
		if err == nil {
			t.Error("Expected error for invalid signature")
		}

		if !strings.Contains(err.Error(), "signature mismatch") {
			t.Errorf("Expected error about signature mismatch, got: %v", err)
		}
	})
}

func TestBuildAuthorizationHeader(t *testing.T) {
	t.Run("BasicAuthHeaderBuild", func(t *testing.T) {
		workerId := "test-worker-123"
		workerToken := "test-token-456"

		req, err := http.NewRequest("GET", "http://localhost:8080/api/v1/test", nil)
		if err != nil {
			t.Fatalf("Failed to create request: %v", err)
		}

		authHeader := BuildAuthorizationHeader(workerId, workerToken, req)

		// Check format
		if !strings.HasPrefix(authHeader, "DISTWORKER1_HMAC_SHA256") {
			t.Error("Expected authorization header to start with DISTWORKER1_HMAC_SHA256")
		}

		// Check components
		if !strings.Contains(authHeader, "workerId="+workerId) {
			t.Errorf("Expected authorization header to contain workerId=%s", workerId)
		}

		if !strings.Contains(authHeader, "SignedHeaders=") {
			t.Error("Expected authorization header to contain SignedHeaders")
		}

		if !strings.Contains(authHeader, "Signature=") {
			t.Error("Expected authorization header to contain Signature")
		}

		// Check that date header was set
		dateHeader := req.Header.Get("x-distworker-date")
		if dateHeader == "" {
			t.Error("Expected x-distworker-date header to be set")
		}

		// Validate date format
		_, err = time.Parse(DateFormat, dateHeader)
		if err != nil {
			t.Errorf("Expected valid date format, got: %v", err)
		}
	})

	t.Run("AuthHeaderValidatesSuccessfully", func(t *testing.T) {
		workerId := "test-worker-123"
		workerToken := "test-token-456"

		req, err := http.NewRequest("GET", "http://localhost:8080/api/v1/test?param=value", nil)
		if err != nil {
			t.Fatalf("Failed to create request: %v", err)
		}

		authHeader := BuildAuthorizationHeader(workerId, workerToken, req)
		req.Header.Set("Authorization", authHeader)

		// The built header should validate successfully
		_, err = validateSignature(workerToken, req)
		if err != nil {
			t.Errorf("Expected built authorization header to validate successfully, got: %v", err)
		}
	})
}

func TestCompleteAuthFlow(t *testing.T) {
	t.Run("EndToEndHTTPAuth", func(t *testing.T) {
		workerId := "worker-12345"
		workerToken := "super-secret-token"

		// Simulate client building a request
		req, err := http.NewRequest("POST", "http://localhost:8080/api/v1/tasks", nil)
		if err != nil {
			t.Fatalf("Failed to create request: %v", err)
		}

		// Add query parameters
		q := req.URL.Query()
		q.Add("queue", "ml/training")
		q.Add("priority", "high")
		req.URL.RawQuery = q.Encode()

		// Client builds authorization header
		authHeader := BuildAuthorizationHeader(workerId, workerToken, req)
		req.Header.Set("Authorization", authHeader)

		// Server validates the signature
		vctx, err := validateSignature(workerToken, req)
		if err != nil {
			t.Errorf("Expected end-to-end auth flow to succeed, got: %v", err)
		}

		// Verify worker ID was extracted
		if vctx.WorkerId != workerId {
			t.Errorf("Expected workerId to be %s, got: %s", workerId, vctx.WorkerId)
		}
	})

	t.Run("EndToEndWebSocketAuth", func(t *testing.T) {
		workerToken := "super-secret-token"
		messageData := []byte(`{"type": "heartbeat", "timestamp": 1661867760000}`)

		// Client generates signature
		now := time.Now().UTC()
		dateStr := now.Format(DateFormat)
		dateOnly := now.Format(DateOnlyFormat)

		signature := GenerateWebSocketSignature(workerToken, dateOnly, messageData)

		// Server validates signature
		err := ValidateWebSocketSignature(workerToken, dateStr, messageData, signature)
		if err != nil {
			t.Errorf("Expected end-to-end WebSocket auth to succeed, got: %v", err)
		}
	})
}

func TestAuthConstants(t *testing.T) {
	t.Run("AuthConstantsAreCorrect", func(t *testing.T) {
		if AuthPrefix != "DISTWORKER1" {
			t.Errorf("Expected AuthPrefix to be 'DISTWORKER1', got: %s", AuthPrefix)
		}

		if SigningKeyData != "distworker1_request" {
			t.Errorf("Expected SigningKeyData to be 'distworker1_request', got: %s", SigningKeyData)
		}

		if WSSigningKeyData != "distworker1_websocket" {
			t.Errorf("Expected WSSigningKeyData to be 'distworker1_websocket', got: %s", WSSigningKeyData)
		}

		// Test date formats
		now := time.Now().UTC()

		dateFormatted := now.Format(DateFormat)
		if len(dateFormatted) != 16 { // 20220830T123600Z
			t.Errorf("Expected DateFormat to produce 16-character string, got: %s", dateFormatted)
		}

		dateOnlyFormatted := now.Format(DateOnlyFormat)
		if len(dateOnlyFormatted) != 8 { // 20220830
			t.Errorf("Expected DateOnlyFormat to produce 8-character string, got: %s", dateOnlyFormatted)
		}
	})
}

func TestEdgeCasesAndErrorHandling(t *testing.T) {
	t.Run("EmptyWorkerToken", func(t *testing.T) {
		// Test with empty worker token
		signature := GenerateSignature("", "20220830", "test canonical request")

		// Should still generate a signature (even if not useful)
		if signature == "" {
			t.Error("Expected signature even with empty worker token")
		}
	})

	t.Run("EmptyCanonicalRequest", func(t *testing.T) {
		signature := GenerateSignature("test-token", "20220830", "")

		// Should still generate a signature
		if signature == "" {
			t.Error("Expected signature even with empty canonical request")
		}
	})

	t.Run("VeryLongInputs", func(t *testing.T) {
		// Test with very long inputs
		longToken := strings.Repeat("a", 1000)
		longCanonical := strings.Repeat("GET\n/very/long/path\n", 100)

		signature := GenerateSignature(longToken, "20220830", longCanonical)

		if signature == "" {
			t.Error("Expected signature even with very long inputs")
		}
	})

	t.Run("UnicodeInInputs", func(t *testing.T) {
		// Test with unicode characters
		unicodeToken := "토큰-🔐-token"
		unicodeCanonical := "GET\n/api/한글/test\n\nx-distworker-date:20220830T123600Z\nx-distworker-date\nUNSIGNED-PAYLOAD"

		signature := GenerateSignature(unicodeToken, "20220830", unicodeCanonical)

		if signature == "" {
			t.Error("Expected signature even with unicode inputs")
		}

		// Should be deterministic
		signature2 := GenerateSignature(unicodeToken, "20220830", unicodeCanonical)
		if signature != signature2 {
			t.Error("Unicode signature should be deterministic")
		}
	})

	t.Run("MalformedAuthorizationHeader", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/test", nil)
		req.Header.Set("x-distworker-date", time.Now().UTC().Format(DateFormat))

		malformedHeaders := []string{
			"DISTWORKER1_HMAC_SHA256 workerId=test",                                // Missing other parts
			"DISTWORKER1_HMAC_SHA256 workerId",                                     // No equals
			"DISTWORKER1_HMAC_SHA256 =test, SignedHeaders=, Signature=",            // Empty key
			"DISTWORKER1_HMAC_SHA256 workerId=test, SignedHeaders=date, Signature", // No equals for signature
		}

		for _, malformed := range malformedHeaders {
			req.Header.Set("Authorization", malformed)
			_, err := validateSignature("test-token", req)
			if err == nil {
				t.Errorf("Expected error for malformed auth header: %s", malformed)
			}
		}
	})
}

func validateSignature(token string, req *http.Request) (*ValidateContext, error) {
	vctx, err := NewValidateContext(req)
	if err != nil {
		return vctx, err
	}
	return vctx, vctx.ValidateSignature(token)
}
