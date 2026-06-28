package invoice

import (
	"testing"
)

func TestProfile_URN(t *testing.T) {
	tests := []struct {
		name     string
		profile  Profile
		expected string
	}{
		{
			name:     "EN16931 profile URN",
			profile:  ProfileEN16931,
			expected: "urn:cen.eu:en16931:2017",
		},
		{
			name:     "EXTENDED profile URN",
			profile:  ProfileEXTENDED,
			expected: "urn:cen.eu:en16931:2017#conformant#urn:factur-x.eu:1p0:extended",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if urn := tt.profile.URN(); urn != tt.expected {
				t.Errorf("URN() = %v, want %v", urn, tt.expected)
			}
		})
	}
}

func TestProfile_IsValid(t *testing.T) {
	tests := []struct {
		name    string
		profile Profile
		valid   bool
	}{
		{"EN16931 is valid", ProfileEN16931, true},
		{"EXTENDED is valid", ProfileEXTENDED, true},
		{"MINIMUM no longer valid", Profile("MINIMUM"), false},
		{"BASIC no longer valid", Profile("BASIC"), false},
		{"Invalid profile", Profile("INVALID"), false},
		{"Empty profile", Profile(""), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.profile.IsValid(); got != tt.valid {
				t.Errorf("IsValid() = %v, want %v", got, tt.valid)
			}
		})
	}
}

func TestDetectProfile_EN16931(t *testing.T) {
	inv := &Invoice{
		Seller: Party{
			Name:    "Vendeur",
			VatID:   "FR12345678901",
			Address: Address{Country: "FR"},
		},
		Totals: Totals{
			VatBreakdown: []VatBreakdown{{Rate: 20, TaxableAmount: 100, VatAmount: 20}},
		},
	}

	if profile := DetectProfile(inv); profile != ProfileEN16931 {
		t.Errorf("DetectProfile() = %v, want %v", profile, ProfileEN16931)
	}
}

func TestDetectProfile_EXTENDED(t *testing.T) {
	inv := &Invoice{
		Seller: Party{
			Name:    "Vendeur",
			VatID:   "FR12345678901",
			Address: Address{Country: "FR"},
			Bank:    &Bank{IBAN: "FR7612345678901234567890123"},
			Contact: &Contact{Email: "contact@example.com"},
		},
		Payment: &Payment{Terms: "30 jours"},
	}

	if profile := DetectProfile(inv); profile != ProfileEXTENDED {
		t.Errorf("DetectProfile() = %v, want %v", profile, ProfileEXTENDED)
	}
}

func TestSetProfile_EN16931_GeneratesVatBreakdown(t *testing.T) {
	inv := &Invoice{
		Lines: []Line{
			{VatRate: 20, TotalExclVat: 100, VatAmount: 20},
		},
	}

	if err := SetProfile(inv, ProfileEN16931); err != nil {
		t.Fatalf("SetProfile(EN16931) failed: %v", err)
	}
	if inv.Profile != ProfileEN16931 {
		t.Errorf("Profile = %v, want EN16931", inv.Profile)
	}
	if len(inv.Totals.VatBreakdown) == 0 {
		t.Error("expected VAT breakdown to be generated")
	}
}

func TestSetProfile_Invalid(t *testing.T) {
	inv := &Invoice{}
	if err := SetProfile(inv, Profile("MINIMUM")); err == nil {
		t.Error("SetProfile(MINIMUM) should fail (profile removed)")
	}
}
