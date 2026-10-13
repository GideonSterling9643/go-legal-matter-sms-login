package legalotp

import (
	"context"
	"testing"
	"time"
)

type fakeCodes struct{ sent, verified bool }

func (f *fakeCodes) SendCode(context.Context, string, string) (OTPResult, error) {
	f.sent = true
	return OTPResult{MessageID: "msg-1"}, nil
}

func (f *fakeCodes) VerifyCode(context.Context, string, string, string) (VerifyResult, error) {
	return VerifyResult{Verified: f.verified}, nil
}

func TestMatterAccessOpensMatterAfterVerifiedCode(t *testing.T) {
	codes := &fakeCodes{verified: true}
	access := NewMatterAccess(codes)
	login := MatterLogin{MatterID: "matter-42", Phone: "+15551234567", ClientName: "Lee", SignedDocument: "engagement.pdf", FollowUpDeadline: time.Date(2026, 10, 3, 9, 0, 0, 0, time.UTC)}
	if _, err := access.RequestCode(context.Background(), login); err != nil {
		t.Fatal(err)
	}
	decision, err := access.Verify(context.Background(), login.MatterID, "481902")
	if err != nil {
		t.Fatal(err)
	}
	if decision.Intake != "open" || decision.SignedDelivery != "ready" || decision.DeadlineFollowUp != "scheduled" {
		t.Fatalf("unexpected decision: %+v", decision)
	}
	if decision.Document != login.SignedDocument || !codes.sent {
		t.Fatalf("unexpected delivery state: %+v", decision)
	}
}
