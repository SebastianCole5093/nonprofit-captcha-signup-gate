package signup

import (
	"context"
	"errors"
	"testing"
)

type verifierStub struct {
	err       error
	calls     int
	lastInput CaptchaInput
}

func (v *verifierStub) Verify(_ context.Context, input CaptchaInput) error {
	v.calls++
	v.lastInput = input
	return v.err
}

func TestRegisterDecision(t *testing.T) {
	tests := []struct {
		name         string
		kind         Kind
		verifyErr    error
		wantErr      bool
		wantReceipt  bool
		wantReminder bool
		wantReport   bool
	}{
		{name: "rejected captcha writes nothing", kind: Donor, verifyErr: &InfraiError{Code: "captcha_rejected", HTTPStatus: 422}, wantErr: true},
		{name: "donor gets receipt", kind: Donor, wantReceipt: true},
		{name: "volunteer gets reminder", kind: Volunteer, wantReminder: true},
		{name: "campaign gets report row", kind: Campaign, wantReport: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			verifier := &verifierStub{err: tt.verifyErr}
			service := NewService(verifier)
			result, err := service.Register(context.Background(), Request{
				RequestID: "req-101", Email: "Data@Helping.test", Name: "Data Team", Kind: tt.kind, WidgetRecordID: "widget-101", CaptchaToken: "browser-token",
			})

			if (err != nil) != tt.wantErr {
				t.Fatalf("Register() error = %v, wantErr %v", err, tt.wantErr)
			}
			if (result.DonorReceipt != nil) != tt.wantReceipt {
				t.Fatalf("receipt present = %v, want %v", result.DonorReceipt != nil, tt.wantReceipt)
			}
			if (result.VolunteerReminder != nil) != tt.wantReminder {
				t.Fatalf("reminder present = %v, want %v", result.VolunteerReminder != nil, tt.wantReminder)
			}
			if (result.CampaignReport != nil) != tt.wantReport {
				t.Fatalf("report present = %v, want %v", result.CampaignReport != nil, tt.wantReport)
			}
			if tt.wantErr && len(service.results) != 0 {
				t.Fatal("rejected captcha must not create a signup")
			}
			if verifier.lastInput.WidgetRecordID != "widget-101" {
				t.Fatalf("widget record ID = %q, want widget-101", verifier.lastInput.WidgetRecordID)
			}
		})
	}
}

func TestRegisterReplaysRequestID(t *testing.T) {
	verifier := &verifierStub{}
	service := NewService(verifier)
	req := Request{RequestID: "req-202", Email: "donor@helping.test", Name: "Donor", Kind: Donor, WidgetRecordID: "widget-202", CaptchaToken: "browser-token"}

	first, err := service.Register(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	second, err := service.Register(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if first.SignupID != second.SignupID || verifier.calls != 1 {
		t.Fatalf("replay = (%q, %d calls), want (%q, 1 call)", second.SignupID, verifier.calls, first.SignupID)
	}
}

func TestClientErrorCanBeInspected(t *testing.T) {
	err := error(&InfraiError{Code: "captcha_rejected", HTTPStatus: 422})
	var target *InfraiError
	if !errors.As(err, &target) || target.HTTPStatus != 422 {
		t.Fatal("expected typed Infrai error")
	}
}
