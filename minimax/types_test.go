package minimax

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestMetadataCountsAcceptBothEncodings verifies that the image counts
// are read whether MiniMax sends them as numbers or as strings.
//
// The live API sends them as strings, although the published schema
// declares them as integers. Rejecting the string form failed the whole
// response, so a successful generation was reported as an invalid
// parameter error and the images it had produced were discarded.
func TestMetadataCountsAcceptBothEncodings(t *testing.T) {
	cases := []struct {
		name        string
		body        string
		wantSuccess int
		wantFailed  int
	}{
		{"as strings, as the API sends them", `{"success_count":"3","failed_count":"1"}`, 3, 1},
		{"as numbers, as the schema declares", `{"success_count":3,"failed_count":1}`, 3, 1},
		{"mixed", `{"success_count":"3","failed_count":1}`, 3, 1},
		{"zero", `{"success_count":"0","failed_count":"0"}`, 0, 0},
		{"empty strings", `{"success_count":"","failed_count":""}`, 0, 0},
		{"null", `{"success_count":null,"failed_count":null}`, 0, 0},
		{"absent", `{}`, 0, 0},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var meta Metadata
			if err := json.Unmarshal([]byte(tc.body), &meta); err != nil {
				t.Fatalf("Unmarshal(%s): %v", tc.body, err)
			}
			if meta.SuccessCount != tc.wantSuccess {
				t.Errorf("SuccessCount = %d, want %d", meta.SuccessCount, tc.wantSuccess)
			}
			if meta.FailedCount != tc.wantFailed {
				t.Errorf("FailedCount = %d, want %d", meta.FailedCount, tc.wantFailed)
			}
		})
	}
}

// TestMetadataRejectsACountThatIsNotANumber verifies that a count that
// is neither a number nor a numeric string is reported as an error,
// naming the field, rather than being silently read as zero.
func TestMetadataRejectsACountThatIsNotANumber(t *testing.T) {
	var meta Metadata
	err := json.Unmarshal([]byte(`{"success_count":"many"}`), &meta)
	if err == nil {
		t.Fatal("expected an error for a count that is not a number")
	}
	if !strings.Contains(err.Error(), "success_count") {
		t.Errorf("error %q does not name the offending field", err)
	}
}

// TestBaseRespAcceptsAStringStatusCode verifies that a stringified
// status code is read as a number.
//
// A mismatched status code would otherwise abort the decode and turn
// every MiniMax error into an unreadable response, hiding the real
// cause of the failure.
func TestBaseRespAcceptsAStringStatusCode(t *testing.T) {
	var resp BaseResp
	if err := json.Unmarshal([]byte(`{"status_code":"1008","status_msg":"insufficient balance"}`), &resp); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if resp.StatusCode != StatusInsufficientBalance {
		t.Errorf("StatusCode = %d, want %d", resp.StatusCode, StatusInsufficientBalance)
	}
	if resp.StatusMsg != "insufficient balance" {
		t.Errorf("StatusMsg = %q", resp.StatusMsg)
	}
}

// TestBaseRespRejectsAStatusCodeThatIsNotANumber verifies that an
// unreadable status code is reported as an error.
func TestBaseRespRejectsAStatusCodeThatIsNotANumber(t *testing.T) {
	var resp BaseResp
	err := json.Unmarshal([]byte(`{"status_code":"nope"}`), &resp)
	if err == nil {
		t.Fatal("expected an error for a status code that is not a number")
	}
	if !strings.Contains(err.Error(), "status_code") {
		t.Errorf("error %q does not name the offending field", err)
	}
}

// TestTheUnreadableResponseCodeIsNotInvalidParams verifies that the
// Aurore-side code does not collide with a code MiniMax documents.
//
// 2013 means MiniMax rejected the request parameters, so reusing it for
// a body this client could not parse would send the reader in the wrong
// direction.
func TestTheUnreadableResponseCodeIsNotInvalidParams(t *testing.T) {
	if StatusUnreadableResponse == StatusInvalidParams {
		t.Fatal("the unreadable response code must not be the invalid params code")
	}
	if StatusUnreadableResponse == StatusOK {
		t.Fatal("the unreadable response code must not be the success code")
	}

	err := &Error{StatusCode: StatusUnreadableResponse}
	if err.IsInvalidParams() {
		t.Error("an unreadable response must not report itself as invalid params")
	}
	if !err.IsUnreadableResponse() {
		t.Error("an unreadable response must report itself as one")
	}
}
