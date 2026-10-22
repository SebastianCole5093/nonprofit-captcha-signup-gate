package signup

import (
	"context"
	"fmt"
	"strings"
	"sync"
)

type Kind string

const (
	Donor     Kind = "donor"
	Volunteer Kind = "volunteer"
	Campaign  Kind = "campaign"
)

type Request struct {
	RequestID      string `json:"request_id"`
	Email          string `json:"email"`
	Name           string `json:"name"`
	Kind           Kind   `json:"kind"`
	WidgetRecordID string `json:"widget_record_id"`
	CaptchaToken   string `json:"captcha_token"`
	IP             string `json:"-"`
}

type DonorReceipt struct {
	ReceiptID string `json:"receipt_id"`
	Email     string `json:"email"`
}

type VolunteerReminder struct {
	Channel string `json:"channel"`
	To      string `json:"to"`
}

type CampaignReport struct {
	CampaignID string `json:"campaign_id"`
	Signups    int    `json:"signups"`
}

type Result struct {
	SignupID          string             `json:"signup_id"`
	Status            string             `json:"status"`
	DonorReceipt      *DonorReceipt      `json:"donor_receipt,omitempty"`
	VolunteerReminder *VolunteerReminder `json:"volunteer_reminder,omitempty"`
	CampaignReport    *CampaignReport    `json:"campaign_report,omitempty"`
}

type ValidationError struct {
	Message string
}

func (e *ValidationError) Error() string { return e.Message }

type Service struct {
	verifier CaptchaVerifier
	mu       sync.Mutex
	results  map[string]Result
	counts   map[Kind]int
}

func NewService(verifier CaptchaVerifier) *Service {
	return &Service{verifier: verifier, results: make(map[string]Result), counts: make(map[Kind]int)}
}

func (s *Service) Register(ctx context.Context, req Request) (Result, error) {
	if req.RequestID == "" || req.Email == "" || req.Name == "" || req.WidgetRecordID == "" || req.CaptchaToken == "" {
		return Result{}, &ValidationError{Message: "request_id, email, name, widget_record_id, and captcha_token are required"}
	}
	if req.Kind != Donor && req.Kind != Volunteer && req.Kind != Campaign {
		return Result{}, &ValidationError{Message: "kind must be donor, volunteer, or campaign"}
	}

	s.mu.Lock()
	previous, exists := s.results[req.RequestID]
	s.mu.Unlock()
	if exists {
		return previous, nil
	}

	err := s.verifier.Verify(ctx, CaptchaInput{
		WidgetRecordID: req.WidgetRecordID,
		Token:          req.CaptchaToken,
		Vendor:         "turnstile",
		IP:             req.IP,
		Action:         "nonprofit_signup",
		ScoreThreshold: 0.7,
	})
	if err != nil {
		return Result{}, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if previous, exists := s.results[req.RequestID]; exists {
		return previous, nil
	}
	s.counts[req.Kind]++
	result := projectResult(req, s.counts[req.Kind])
	s.results[req.RequestID] = result
	return result, nil
}

func projectResult(req Request, sequence int) Result {
	id := fmt.Sprintf("signup-%s", req.RequestID)
	result := Result{SignupID: id, Status: "accepted"}
	switch req.Kind {
	case Donor:
		result.DonorReceipt = &DonorReceipt{ReceiptID: fmt.Sprintf("receipt-%s", req.RequestID), Email: strings.ToLower(req.Email)}
	case Volunteer:
		result.VolunteerReminder = &VolunteerReminder{Channel: "email", To: strings.ToLower(req.Email)}
	case Campaign:
		result.CampaignReport = &CampaignReport{CampaignID: req.RequestID, Signups: sequence}
	}
	return result
}
