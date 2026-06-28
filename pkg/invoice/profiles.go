package invoice

import (
	"fmt"
)

// Profile représente un profil Facture-X
type Profile string

const (
	// ProfileEN16931 - Profil standard conforme EN16931
	ProfileEN16931 Profile = "EN16931"

	// ProfileEXTENDED - Profil étendu avec tous les champs
	ProfileEXTENDED Profile = "EXTENDED"
)

// ProfileURN retourne l'URN Facture-X du profil
func (p Profile) URN() string {
	switch p {
	case ProfileEN16931:
		return "urn:cen.eu:en16931:2017"
	case ProfileEXTENDED:
		return "urn:cen.eu:en16931:2017#conformant#urn:factur-x.eu:1p0:extended"
	default:
		return string(p)
	}
}

// IsValid vérifie si le profil est valide
func (p Profile) IsValid() bool {
	switch p {
	case ProfileEN16931, ProfileEXTENDED:
		return true
	default:
		return false
	}
}

// ProfileRequirements définit les champs obligatoires par profil
type ProfileRequirements struct {
	RequireVatID            bool
	RequireAddress          bool
	RequireVatBreakdown     bool
	RequireLineDetails      bool
	RequirePaymentTerms     bool
	RequireBankInfo         bool
	RequireContact          bool
	RequireRegistration     bool
	AllowMultipleVATRates   bool
	AllowCharges            bool
	AllowAllowances         bool
	AllowDocumentReferences bool
	AllowBillingPeriod      bool
	MaxLines                int // 0 = illimité
}

// GetRequirements retourne les exigences d'un profil
func (p Profile) GetRequirements() ProfileRequirements {
	switch p {
	case ProfileEN16931:
		return ProfileRequirements{
			RequireVatID:            true,
			RequireAddress:          true,
			RequireVatBreakdown:     true,
			RequireLineDetails:      true,
			RequirePaymentTerms:     false,
			RequireBankInfo:         false,
			RequireContact:          false,
			RequireRegistration:     false,
			AllowMultipleVATRates:   true,
			AllowCharges:            true,
			AllowAllowances:         true,
			AllowDocumentReferences: true,
			AllowBillingPeriod:      true,
			MaxLines:                0,
		}
	case ProfileEXTENDED:
		return ProfileRequirements{
			RequireVatID:            true,
			RequireAddress:          true,
			RequireVatBreakdown:     true,
			RequireLineDetails:      true,
			RequirePaymentTerms:     true,
			RequireBankInfo:         true,
			RequireContact:          true,
			RequireRegistration:     false,
			AllowMultipleVATRates:   true,
			AllowCharges:            true,
			AllowAllowances:         true,
			AllowDocumentReferences: true,
			AllowBillingPeriod:      true,
			MaxLines:                0,
		}
	default:
		return ProfileRequirements{}
	}
}

// ValidateProfile valide une facture selon son profil
func ValidateProfile(inv *Invoice) error {
	profile := Profile(inv.Profile)

	if !profile.IsValid() {
		return fmt.Errorf("invalid profile: %s (valid: EN16931, EXTENDED)", inv.Profile)
	}

	req := profile.GetRequirements()

	// Validation Identifiant Vendeur (TVA ou SIRET)
	if req.RequireVatID {
		if inv.Seller.VatID == "" && inv.Seller.GlobalID == nil {
			return fmt.Errorf("profile %s requires seller VAT ID or Global ID (SIRET)", profile)
		}
		// On ne rend plus obligatoire le VAT ID du client car il peut s'agir de B2C ou d'entités non assujetties
	}

	// Validation adresse
	if req.RequireAddress {
		if inv.Seller.Address.Country == "" {
			return fmt.Errorf("profile %s requires seller address with country", profile)
		}
		if inv.Buyer.Address.Country == "" {
			return fmt.Errorf("profile %s requires buyer address with country", profile)
		}
	}

	// Validation détails lignes
	if req.RequireLineDetails {
		for i, line := range inv.Lines {
			if line.Description == "" {
				return fmt.Errorf("profile %s requires line %d description", profile, i+1)
			}
			if line.Quantity == 0 {
				return fmt.Errorf("profile %s requires line %d quantity", profile, i+1)
			}
		}
	}

	// Validation conditions de paiement (EXTENDED)
	if req.RequirePaymentTerms {
		if inv.Payment == nil || inv.Payment.Terms == "" {
			return fmt.Errorf("profile %s requires payment terms", profile)
		}
	}

	// Validation informations bancaires (EXTENDED)
	if req.RequireBankInfo {
		if inv.Seller.Bank == nil || inv.Seller.Bank.IBAN == "" {
			return fmt.Errorf("profile %s requires seller bank information", profile)
		}
	}

	// Validation contact (EXTENDED)
	if req.RequireContact {
		if inv.Seller.Contact == nil {
			return fmt.Errorf("profile %s requires seller contact information", profile)
		}
	}

	// Validation VatBreakdown (EN16931 et EXTENDED)
	if req.RequireVatBreakdown {
		if len(inv.Totals.VatBreakdown) == 0 {
			return fmt.Errorf("profile %s requires VAT breakdown", profile)
		}
	}

	// Limitation nombre de lignes (MINIMUM)
	if req.MaxLines > 0 && len(inv.Lines) > req.MaxLines {
		return fmt.Errorf("profile %s allows maximum %d lines, got %d", profile, req.MaxLines, len(inv.Lines))
	}

	return nil
}

// DetectProfile détecte automatiquement le profil approprié
func DetectProfile(inv *Invoice) Profile {
	// Si un profil est déjà défini et valide, le garder
	if inv.Profile != "" {
		profile := Profile(inv.Profile)
		if profile.IsValid() {
			return profile
		}
	}

	// Champs étendus -> EXTENDED, sinon EN16931 par défaut.
	hasExtendedFields := (inv.Payment != nil && inv.Payment.Terms != "") ||
		(inv.Seller.Bank != nil && inv.Seller.Bank.IBAN != "") ||
		(inv.Seller.Contact != nil)

	if hasExtendedFields {
		return ProfileEXTENDED
	}
	return ProfileEN16931
}

// SetProfile définit le profil cible s'il est valide (EN16931 ou EXTENDED).
// Pour EN16931, génère automatiquement le détail de TVA s'il est absent.
func SetProfile(inv *Invoice, targetProfile Profile) error {
	if !targetProfile.IsValid() {
		return fmt.Errorf("invalid target profile: %s", targetProfile)
	}

	if len(inv.Totals.VatBreakdown) == 0 {
		breakdown := make(map[float64]VatBreakdown)
		for _, line := range inv.Lines {
			if existing, ok := breakdown[line.VatRate]; ok {
				existing.TaxableAmount += line.TotalExclVat
				existing.VatAmount += line.VatAmount
				breakdown[line.VatRate] = existing
			} else {
				breakdown[line.VatRate] = VatBreakdown{
					Rate:          line.VatRate,
					TaxableAmount: line.TotalExclVat,
					VatAmount:     line.VatAmount,
				}
			}
		}
		inv.Totals.VatBreakdown = make([]VatBreakdown, 0, len(breakdown))
		for _, vb := range breakdown {
			inv.Totals.VatBreakdown = append(inv.Totals.VatBreakdown, vb)
		}
	}

	if targetProfile == ProfileEXTENDED {
		if inv.Payment == nil {
			return fmt.Errorf("cannot use EXTENDED: payment information required")
		}
		if inv.Seller.Bank == nil {
			return fmt.Errorf("cannot use EXTENDED: seller bank information required")
		}
		if inv.Seller.Contact == nil {
			return fmt.Errorf("cannot use EXTENDED: seller contact required")
		}
	}

	inv.Profile = targetProfile
	return nil
}
