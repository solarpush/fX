export type FacturXProfile = 'EN16931' | 'EXTENDED';

export interface Invoice {
  version?: string;
  profile: FacturXProfile;
  invoice: Details;
  seller: Party;
  buyer: Party;
  lines: Line[];
  totals: Totals;
  payment?: Payment;
  allowance_charges?: AllowanceCharge[];
  billing_period?: BillingPeriod;
  document_references?: DocumentReference[];
  notes?: Note[];
}

export interface Details {
  number: string;
  issue_date: string;
  due_date?: string;
  currency?: string;
  type?: string;
  note?: string;
  buyer_reference?: string;
  purchase_order_ref?: string;
  contract_ref?: string;
  preceding_invoice_ref?: string;
}

export interface Party {
  name: string;
  registration?: string;
  vat_id?: string;
  contact?: Contact;
  address: Address;
  bank?: Bank;
  global_id?: GlobalID;
}

export interface Contact {
  email?: string;
  phone?: string;
}

export interface Address {
  street: string;
  postal_code: string;
  city: string;
  country: string;
}

export interface Bank {
  iban?: string;
  bic?: string;
  bank_name?: string;
  account_name?: string;
}

export interface GlobalID {
  scheme_id: string;
  value: string;
}

export interface Line {
  id: string;
  description: string;
  quantity: number;
  unit?: string;
  unit_price: number;
  vat_rate: number;
  vat_amount?: number;
  total_excl_vat?: number;
  total_incl_vat?: number;
  allowance_charges?: AllowanceCharge[];
  product_code?: string;
  product_code_scheme?: string;
  buyer_product_code?: string;
  seller_product_code?: string;
  order_line_reference?: string;
}

export interface Totals {
  subtotal_excl_vat: number;
  allowance_total?: number;
  charge_total?: number;
  tax_basis_total?: number;
  vat_breakdown: VatBreakdown[];
  total_vat: number;
  total_incl_vat: number;
  amount_due: number;
  prepaid_amount?: number;
  rounding_amount?: number;
}

export interface VatBreakdown {
  rate: number;
  taxable_amount: number;
  vat_amount: number;
}

export interface Payment {
  terms?: string;
  method?: string;
  iban?: string;
  due_date?: string;
  payment_means?: PaymentMeans;
  reference?: string;
  early_discount?: Discount;
}

export interface PaymentMeans {
  type_code: string;
  information?: string;
  payee_account?: Bank;
  payment_reference?: string;
}

export interface Discount {
  percent: number;
  base_amount?: number;
  amount?: number;
  days_from?: number;
  until_date?: string;
}

export interface AllowanceCharge {
  is_charge: boolean;
  reason?: string;
  reason_code?: string;
  amount: number;
  base_amount?: number;
  percent?: number;
  vat_rate?: number;
  vat_amount?: number;
  vat_category_code?: string;
}

export interface BillingPeriod {
  start_date: string;
  end_date: string;
  description?: string;
}

export interface DocumentReference {
  id: string;
  type_code?: string;
  issue_date?: string;
}

export interface Note {
  content: string;
  subject_code?: string;
}
