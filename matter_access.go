package legalotp

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"sync"
	"time"
)

type CodeService interface {
	SendCode(context.Context, string, string) (OTPResult, error)
	VerifyCode(context.Context, string, string, string) (VerifyResult, error)
}

type MatterLogin struct {
	MatterID         string    `json:"matter_id"`
	Phone            string    `json:"phone"`
	ClientName       string    `json:"client_name"`
	SignedDocument   string    `json:"signed_document"`
	FollowUpDeadline time.Time `json:"follow_up_deadline"`
}

type AccessDecision struct {
	MatterID         string `json:"matter_id"`
	Intake           string `json:"intake"`
	SignedDelivery   string `json:"signed_delivery"`
	DeadlineFollowUp string `json:"deadline_follow_up"`
	Document         string `json:"document"`
	Deadline         string `json:"deadline"`
}

type MatterAccess struct {
	codes   CodeService
	mu      sync.RWMutex
	pending map[string]MatterLogin
}

func NewMatterAccess(codes CodeService) *MatterAccess {
	return &MatterAccess{codes: codes, pending: make(map[string]MatterLogin)}
}

func (m *MatterAccess) RequestCode(ctx context.Context, login MatterLogin) (OTPResult, error) {
	if login.MatterID == "" || login.Phone == "" || login.ClientName == "" || login.SignedDocument == "" || login.FollowUpDeadline.IsZero() {
		return OTPResult{}, errors.New("matter_id, phone, client_name, signed_document, and follow_up_deadline are required")
	}
	result, err := m.codes.SendCode(ctx, login.Phone, stableKey(login.MatterID, "request"))
	if err != nil {
		return OTPResult{}, err
	}
	m.mu.Lock()
	m.pending[login.MatterID] = login
	m.mu.Unlock()
	return result, nil
}

func (m *MatterAccess) Verify(ctx context.Context, matterID, code string) (AccessDecision, error) {
	m.mu.RLock()
	login, found := m.pending[matterID]
	m.mu.RUnlock()
	if !found {
		return AccessDecision{}, errors.New("matter login not found")
	}
	result, err := m.codes.VerifyCode(ctx, login.Phone, code, stableKey(matterID, "verify"))
	if err != nil {
		return AccessDecision{}, err
	}
	if !result.Verified {
		return AccessDecision{}, errors.New("code was not verified")
	}

	m.mu.Lock()
	delete(m.pending, matterID)
	m.mu.Unlock()
	return AccessDecision{
		MatterID:         matterID,
		Intake:           "open",
		SignedDelivery:   "ready",
		DeadlineFollowUp: "scheduled",
		Document:         login.SignedDocument,
		Deadline:         login.FollowUpDeadline.UTC().Format(time.RFC3339),
	}, nil
}

func stableKey(matterID, phase string) string {
	sum := sha256.Sum256([]byte(matterID + ":" + phase))
	return "matter-" + hex.EncodeToString(sum[:12])
}
