package invoice

import "time"

// Invoice represents a complete invoice structure
type Invoice struct {
	Version            string              `json:"version"`
	Profile            Profile             `json:"profile"`
	Invoice            Details             `json:"invoice"`
	Seller             Party               `json:"seller"`
	Buyer              Party               `json:"buyer"`
	Lines              []Line              `json:"lines"`
	Totals             Totals              `json:"totals"`
	Payment            *Payment            `json:"payment,omitempty"`
	AllowanceCharges   []AllowanceCharge   `json:"allowance_charges,omitempty"`   // Charges/réductions niveau document
	BillingPeriod      *BillingPeriod      `json:"billing_period,omitempty"`      // Période de facturation
	DocumentReferences []DocumentReference `json:"document_references,omitempty"` // Références (commande, BL, etc.)
	Notes              []Note              `json:"notes,omitempty"`               // Notes structurées
}

// Details contains invoice metadata
type Details struct {
	Number              string    `json:"number"`
	IssueDate           time.Time `json:"issue_date"`
	DueDate             time.Time `json:"due_date,omitempty"`
	Currency            string    `json:"currency"`
	Type                string    `json:"type"` // 380=Invoice, 381=Credit note, 384=Corrected, 389=Self-billed, 751=Information
	Note                string    `json:"note,omitempty"`
	BusinessProcess     string    `json:"business_process,omitempty"`      // BT-23 (ex: A1). Défaut: A1
	BuyerReference      string    `json:"buyer_reference,omitempty"`       // Référence acheteur
	PurchaseOrderRef    string    `json:"purchase_order_ref,omitempty"`    // Numéro de commande
	ContractRef         string    `json:"contract_ref,omitempty"`          // Numéro de contrat
	PrecedingInvoiceRef string    `json:"preceding_invoice_ref,omitempty"` // Facture précédente (avoirs/rectificatives)
}

// Party represents seller or buyer
type Party struct {
	Name         string    `json:"name"`
	Registration string    `json:"registration,omitempty"`
	VatID        string    `json:"vat_id,omitempty"`
	Contact      *Contact  `json:"contact,omitempty"`
	Address      Address   `json:"address"`
	Bank         *Bank     `json:"bank,omitempty"`
	GlobalID     *GlobalID `json:"global_id,omitempty"` // Identifiants normalisés (GLN, SIRET, etc.)
}

// Contact contains contact information
type Contact struct {
	Email string `json:"email,omitempty"`
	Phone string `json:"phone,omitempty"`
}

// Address represents a postal address
type Address struct {
	Street     string `json:"street"`
	PostalCode string `json:"postal_code"`
	City       string `json:"city"`
	Country    string `json:"country"` // ISO 3166-1 alpha-2
}

// Bank contains banking information
type Bank struct {
	IBAN        string `json:"iban,omitempty"`
	BIC         string `json:"bic,omitempty"`
	BankName    string `json:"bank_name,omitempty"`    // Nom de la banque
	AccountName string `json:"account_name,omitempty"` // Titulaire du compte
}

// GlobalID represents normalized party identifiers
type GlobalID struct {
	SchemeID string `json:"scheme_id"` // 0088=GLN, 0009=SIRET, 0060=DUNS, etc.
	Value    string `json:"value"`
}

// Line represents an invoice line item
type Line struct {
	ID                 string            `json:"id"`
	Description        string            `json:"description"`
	Quantity           float64           `json:"quantity"`
	Unit               string            `json:"unit,omitempty"`
	UnitPrice          float64           `json:"unit_price"`
	VatRate            float64           `json:"vat_rate"`
	VatAmount          float64           `json:"vat_amount"`
	TotalExclVat       float64           `json:"total_excl_vat"`
	TotalInclVat       float64           `json:"total_incl_vat"`
	AllowanceCharges   []AllowanceCharge `json:"allowance_charges,omitempty"`    // Charges/réductions niveau ligne
	ProductCode        string            `json:"product_code,omitempty"`         // Code article
	ProductCodeScheme  string            `json:"product_code_scheme,omitempty"`  // Schéma (GTIN, EAN, etc.)
	BuyerProductCode   string            `json:"buyer_product_code,omitempty"`   // Référence acheteur
	SellerProductCode  string            `json:"seller_product_code,omitempty"`  // Référence vendeur
	OrderLineReference string            `json:"order_line_reference,omitempty"` // Référence ligne commande
}

// Totals contains invoice totals and VAT breakdown
type Totals struct {
	SubtotalExclVat float64        `json:"subtotal_excl_vat"`
	AllowanceTotal  float64        `json:"allowance_total,omitempty"` // Total réductions
	ChargeTotal     float64        `json:"charge_total,omitempty"`    // Total charges additionnelles
	TaxBasisTotal   float64        `json:"tax_basis_total,omitempty"` // Base taxable (après charges/réductions)
	VatBreakdown    []VatBreakdown `json:"vat_breakdown"`
	TotalVat        float64        `json:"total_vat"`
	TotalInclVat    float64        `json:"total_incl_vat"`
	AmountDue       float64        `json:"amount_due"`
	PrepaidAmount   float64        `json:"prepaid_amount,omitempty"`  // Montant déjà payé
	RoundingAmount  float64        `json:"rounding_amount,omitempty"` // Arrondi
}

// VatBreakdown represents VAT calculation for a specific rate
type VatBreakdown struct {
	Rate          float64 `json:"rate"`
	TaxableAmount float64 `json:"taxable_amount"`
	VatAmount     float64 `json:"vat_amount"`
}

// Payment contains payment terms and information
type Payment struct {
	Terms         string        `json:"terms,omitempty"`
	Method        string        `json:"method,omitempty"`
	IBAN          string        `json:"iban,omitempty"`
	DueDate       time.Time     `json:"due_date,omitempty"`       // Date d'échéance
	PaymentMeans  *PaymentMeans `json:"payment_means,omitempty"`  // Moyen de paiement structuré
	Reference     string        `json:"reference,omitempty"`      // Référence de paiement
	EarlyDiscount *Discount     `json:"early_discount,omitempty"` // Escompte paiement anticipé
}

// PaymentMeans represents structured payment method information
type PaymentMeans struct {
	TypeCode         string `json:"type_code"`                   // 30=virement, 58=SEPA, 48=carte, 1=espèces
	Information      string `json:"information,omitempty"`       // Information textuelle
	PayeeAccount     *Bank  `json:"payee_account,omitempty"`     // Compte bénéficiaire
	PaymentReference string `json:"payment_reference,omitempty"` // Référence structurée
}

// Discount represents early payment discount
type Discount struct {
	Percent    float64   `json:"percent"`               // Pourcentage de remise
	BaseAmount float64   `json:"base_amount,omitempty"` // Montant de base
	Amount     float64   `json:"amount,omitempty"`      // Montant de la remise
	DaysFrom   int       `json:"days_from,omitempty"`   // Jours depuis émission
	UntilDate  time.Time `json:"until_date,omitempty"`  // Date limite
}

// AllowanceCharge represents charges (frais) or allowances (réductions)
type AllowanceCharge struct {
	IsCharge        bool    `json:"is_charge"`                   // true=charge (frais), false=allowance (réduction)
	Reason          string  `json:"reason,omitempty"`            // Motif
	ReasonCode      string  `json:"reason_code,omitempty"`       // Code motif (AA=publicité, ABL=livraison, etc.)
	Amount          float64 `json:"amount"`                      // Montant
	BaseAmount      float64 `json:"base_amount,omitempty"`       // Base de calcul
	Percent         float64 `json:"percent,omitempty"`           // Pourcentage (si applicable)
	VatRate         float64 `json:"vat_rate,omitempty"`          // Taux TVA applicable
	VatAmount       float64 `json:"vat_amount,omitempty"`        // Montant TVA
	VatCategoryCode string  `json:"vat_category_code,omitempty"` // Code catégorie TVA (S, Z, E, etc.)
}

// BillingPeriod represents the invoicing period
type BillingPeriod struct {
	StartDate   time.Time `json:"start_date"`
	EndDate     time.Time `json:"end_date"`
	Description string    `json:"description,omitempty"` // Description période (ex: "Janvier 2026")
}

// DocumentReference represents a reference to another document
type DocumentReference struct {
	ID            string    `json:"id"`                       // Numéro du document
	TypeCode      string    `json:"type_code,omitempty"`      // Code type (130=commande, 50=BL, etc.)
	IssueDate     time.Time `json:"issue_date,omitempty"`     // Date d'émission
	LineID        string    `json:"line_id,omitempty"`        // Référence ligne (si applicable)
	Description   string    `json:"description,omitempty"`    // Description
	AttachmentURI string    `json:"attachment_uri,omitempty"` // URI pièce jointe
}

// Note represents a structured note/comment
type Note struct {
	Content     string `json:"content"`                // Contenu de la note
	SubjectCode string `json:"subject_code,omitempty"` // AAI=info générale, REG=réglementaire, ABL=conditions
}

// DocumentTypeCode constants
const (
	TypeInvoice            = "380" // Facture commerciale
	TypeCreditNote         = "381" // Avoir
	TypeDebitNote          = "383" // Note de débit
	TypeCorrectedInvoice   = "384" // Facture rectificative
	TypeSelfBilledInvoice  = "389" // Auto-facturation
	TypeInformationInvoice = "751" // Facture d'information
)

// PaymentMeansCode constants
const (
	PaymentMeansCash        = "1"  // Espèces
	PaymentMeansCheque      = "20" // Chèque
	PaymentMeansTransfer    = "30" // Virement bancaire
	PaymentMeansCard        = "48" // Carte de paiement
	PaymentMeansSEPA        = "58" // Prélèvement SEPA
	PaymentMeansDirectDebit = "49" // Prélèvement automatique
)

// AllowanceChargeReasonCode constants
const (
	AllowanceDiscount       = "95"  // Remise commerciale
	AllowanceBonusGoods     = "100" // Marchandises gratuites
	AllowanceVolumeDiscount = "AJ"  // Remise quantité
	ChargeDelivery          = "ABL" // Frais de livraison
	ChargePackaging         = "ABK" // Frais d'emballage
	ChargeInsurance         = "DL"  // Assurance
	ChargeAdministration    = "FC"  // Frais administratifs
)
