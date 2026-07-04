package invoice

import (
	"regexp"
	"sort"
	"strings"
)

// Ce fichier centralise les listes de valeurs autorisées (enums) définies par la
// norme Factur-X / EN16931 ainsi que les helpers de validation de format associés.
// Elles sont exposées dans les erreurs de l'API afin d'indiquer clairement au client
// les valeurs possibles pour chaque champ.

// UnitCode représente un code d'unité de mesure UN/ECE Rec 20
type UnitCode string

// Principaux codes d'unité de mesure
const (
	UnitPiece      UnitCode = "H87"
	UnitOne        UnitCode = "C62"
	UnitEach       UnitCode = "EA"
	UnitHour       UnitCode = "HUR"
	UnitDay        UnitCode = "DAY"
	UnitMonth      UnitCode = "MON"
	UnitKilogram   UnitCode = "KGM"
	UnitLitre      UnitCode = "LTR"
	UnitCubicMeter UnitCode = "MTQ"
	UnitSet        UnitCode = "SET"
	UnitMeter      UnitCode = "MTR"
)

// GlobalIDSchemes liste les schémas d'identifiant normalisé (ISO 6523 ICD) acceptés
// pour Seller/Buyer GlobalID (BT-29/BT-46) et l'adresse électronique (BT-34/BT-49).
// La clé est le code schemeID, la valeur une description lisible.
var GlobalIDSchemes = map[string]string{
	"0002": "SIREN (France)",
	"0009": "SIRET (France)",
	"0060": "DUNS",
	"0088": "GLN (EAN)",
	"0199": "LEI",
	"0225": "SIRET avec établissement",
}

// numericSchemeLengths indique la longueur exacte (en chiffres) attendue pour certains
// schémas numériques. Les schémas absents ne subissent pas de contrôle de longueur.
var numericSchemeLengths = map[string]int{
	"0002": 9,  // SIREN
	"0009": 14, // SIRET
	"0060": 9,  // DUNS
	"0088": 13, // GLN
}

// DocumentTypeCodes liste les codes de type de document (BT-3) supportés.
var DocumentTypeCodes = map[DocumentTypeCode]string{
	TypeInvoice:            "Facture commerciale",
	TypeCreditNote:         "Avoir",
	TypeDebitNote:          "Note de débit",
	TypeCorrectedInvoice:   "Facture rectificative",
	TypeSelfBilledInvoice:  "Auto-facturation",
	TypeInformationInvoice: "Facture d'information",
}

// PaymentMeansCodes liste les codes de moyen de paiement (BT-81, UNCL4461) supportés.
var PaymentMeansCodes = map[PaymentMeansCode]string{
	PaymentMeansCash:  "Espèces",
	"10": "Espèces (en compte)",
	PaymentMeansCheque: "Chèque",
	PaymentMeansTransfer: "Virement bancaire",
	"42": "Virement bancaire (compte de dépôt)",
	PaymentMeansCard: "Carte de paiement",
	PaymentMeansDirectDebit: "Prélèvement automatique",
	"57": "Accord de prélèvement préétabli",
	PaymentMeansSEPA: "Prélèvement SEPA",
	"59": "Prélèvement bancaire au profit du débiteur",
	"97": "Compensation entre parties",
}

// VATCategoryCodes liste les codes de catégorie de TVA (BT-118/BT-151, UNCL5305) supportés.
var VATCategoryCodes = map[string]string{
	"S":  "Taux normal",
	"Z":  "Taux zéro",
	"E":  "Exonéré de TVA",
	"AE": "Autoliquidation (reverse charge)",
	"K":  "Livraison intracommunautaire",
	"G":  "Export hors UE",
	"O":  "Hors champ de la TVA",
	"L":  "Îles Canaries (IGIC)",
	"M":  "Ceuta et Melilla (IPSI)",
}

// ProductCodeSchemes liste les schémas de code article (BT-157) courants.
var ProductCodeSchemes = map[string]string{
	"0160": "GTIN",
	"GTIN": "GTIN",
	"EAN":  "EAN",
	"0088": "GLN",
}

// NoteSubjectCodes liste les codes sujet de note (BT-21, UNCL4451) courants.
var NoteSubjectCodes = map[string]string{
	"AAI": "Information générale",
	"REG": "Information réglementaire",
	"ABL": "Renseignements légaux",
	"AAB": "Conditions d'escompte",
	"PMD": "Pénalités de retard",
	"PMT": "Indemnité forfaitaire de recouvrement",
	"AAK": "Instructions de paiement",
	"TXD": "Information TVA",
	"SUR": "Informations complémentaires",
}

// allowedValues retourne les clés d'une map d'enum, triées, avec leur description
// (ex: "0009 (SIRET (France))") pour être exposées dans les messages d'erreur API.
func allowedValues[K ~string](enum map[K]string) []string {
	keys := make([]string, 0, len(enum))
	for k := range enum {
		keys = append(keys, string(k))
	}
	sort.Strings(keys)
	out := make([]string, 0, len(keys))
	for _, k := range keys {
		out = append(out, k+" ("+enum[K(k)]+")")
	}
	return out
}

var digitsOnly = regexp.MustCompile(`^[0-9]+$`)

// isKnownScheme indique si le code de schéma GlobalID est reconnu.
func isKnownScheme(scheme string) bool {
	_, ok := GlobalIDSchemes[strings.TrimSpace(scheme)]
	return ok
}

// validateGlobalIDValue vérifie le format de la valeur selon le schéma (longueur, chiffres).
// Retourne un message d'erreur non vide si le format est invalide, sinon "".
func validateGlobalIDValue(scheme, value string) string {
	scheme = strings.TrimSpace(scheme)
	value = strings.TrimSpace(value)
	expectedLen, hasLen := numericSchemeLengths[scheme]
	if !hasLen {
		return ""
	}
	if !digitsOnly.MatchString(value) {
		return "value must contain only digits for scheme " + scheme
	}
	if len(value) != expectedLen {
		return "value must be exactly " + itoa(expectedLen) + " digits for scheme " + scheme
	}
	return ""
}

// itoa est un petit helper pour éviter d'importer strconv uniquement pour un int.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		b[i] = '-'
	}
	return string(b[i:])
}
