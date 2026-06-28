import { HttpClient } from '@angular/common/http';
import { Injectable, inject } from '@angular/core';
import { Observable, map } from 'rxjs';
import { environment } from '../../../environments/environment';
import { Invoice, Template } from '../models';

// Réponse wrapper du backend Go
interface ApiResponse<T> {
  success: boolean;
  data: T;
  meta?: { timestamp: string };
  error?: string;
}

interface TemplateListData {
  templates: Template[];
  total: number;
}

// Réponse du backend pour un template
interface TemplateApiResponse {
  id: string;
  name: string;
  type: string;
  content?: string; // Pour les templates .typ
  updated: string;
}

interface GenerateRequest {
  invoice: Invoice;
  templateId?: string;
}

interface GenerateResponse {
  pdfData: string; // Base64 encoded PDF
  metadata?: any;
}

interface ValidationResult {
  valid: boolean;
  errors: Array<{ message: string; field?: string }>;
  profile: string;
}

interface ExtractionResult {
  invoice: Invoice;
  xml: string;
}

@Injectable({
  providedIn: 'root',
})
export class Api {
  private readonly http = inject(HttpClient);
  private readonly baseUrl = environment.apiUrl;

  // Generation endpoints
  generateFacturX(request: GenerateRequest): Observable<Blob> {
    const payload = {
      invoice: request.invoice,
      options: request.templateId ? { templateId: request.templateId } : undefined
    };
    return this.http.post<ApiResponse<GenerateResponse>>(`${this.baseUrl}/generate`, payload).pipe(
      map(res => {
        if (!res.data?.pdfData) {
          throw new Error("No PDF data returned. Dump: " + JSON.stringify(res));
        }
        const byteCharacters = atob(res.data.pdfData);
        const byteNumbers = new Array(byteCharacters.length);
        for (let i = 0; i < byteCharacters.length; i++) {
          byteNumbers[i] = byteCharacters.charCodeAt(i);
        }
        const byteArray = new Uint8Array(byteNumbers);
        return new Blob([byteArray], {type: 'application/pdf'});
      })
    );
  }

  generatePDF(request: GenerateRequest): Observable<Blob> {
    return this.http.post(`${this.baseUrl}/generate/pdf`, request, { responseType: 'blob' });
  }

  generateXML(invoice: Invoice): Observable<string> {
    return this.http.post(`${this.baseUrl}/generate/xml`, invoice, { responseType: 'text' });
  }

  // Validation endpoint
  validate(file: File): Observable<ValidationResult> {
    const formData = new FormData();
    formData.append('file', file);
    return this.http.post<ValidationResult>(`${this.baseUrl}/validate`, formData);
  }

  // Extraction endpoint
  extract(file: File): Observable<ExtractionResult> {
    const formData = new FormData();
    formData.append('file', file);
    return this.http.post<ExtractionResult>(`${this.baseUrl}/extract`, formData);
  }

  // Template endpoints
  listTemplates(): Observable<Template[]> {
    return this.http
      .get<ApiResponse<TemplateListData>>(`${this.baseUrl}/templates`)
      .pipe(map((res) => res.data?.templates || []));
  }

  getTemplate(id: string): Observable<Template> {
    return this.http.get<ApiResponse<TemplateApiResponse>>(`${this.baseUrl}/templates/${encodeURIComponent(id)}`).pipe(
      map((res) => {
        const data = res.data;
        return {
          id: data.id,
          name: data.name,
          type: data.type,
          content: data.content,
          updatedAt: new Date(data.updated),
        } as Template;
      }),
    );
  }

  createTemplate(template: {
    name: string;
    type?: string;
    content?: string;
  }): Observable<Template> {
    return this.http
      .post<ApiResponse<Template>>(`${this.baseUrl}/templates`, template)
      .pipe(map((res) => res.data));
  }

  updateTemplate(id: string, template: Partial<Template>): Observable<Template> {
    return this.http
      .put<ApiResponse<Template>>(`${this.baseUrl}/templates/${encodeURIComponent(id)}`, template)
      .pipe(map((res) => res.data));
  }

  deleteTemplate(id: string): Observable<void> {
    return this.http.delete<void>(`${this.baseUrl}/templates/${encodeURIComponent(id)}`);
  }

  compilePreview(typstCode: string, profile: string = 'EN16931', invoiceData?: any): Observable<Blob> {
    // Données de facture par défaut pour le preview
    const defaultData: any = {
      profile: profile,
      invoice: {
        number: 'PREV-001',
        issueDate: new Date().toISOString().split('T')[0],
        typeCode: '380',
        currencyCode: 'EUR',
      },
      seller: {
        name: 'Entreprise Exemple',
        address: {
          line1: '123 Rue Example',
          city: 'Paris',
          postalCode: '75001',
          countryCode: 'FR',
        },
        taxId: 'FR12345678901',
      },
      buyer: {
        name: 'Client Exemple',
        address: { line1: '456 Avenue Test', city: 'Lyon', postalCode: '69001', countryCode: 'FR' },
        taxId: 'FR98765432109',
      },
      lines: [
        {
          description: 'Article de démonstration',
          quantity: 1,
          unitPrice: 100,
          vatRate: 20,
          vatAmount: 20,
          netAmount: 100,
        },
      ],
      totals: {
        netTotal: 100,
        vatTotal: 20,
        grossTotal: 120,
        dueAmount: 120,
      },
    };

    // Ajouter les champs spécifiques au profil EXTENDED ou si demandé par les capacités
    if (profile === 'EXTENDED' || typstCode.includes('bank') || typstCode.includes('payment')) {
      defaultData.seller.bank = {
        iban: 'FR7630001000011234567890123',
        bic: 'SOCGFRPP',
        bankName: 'Société Générale',
        accountName: 'Entreprise Exemple'
      };
      defaultData.payment = {
        terms: 'Paiement à 30 jours fin de mois.',
        method: 'Virement bancaire',
        dueDate: new Date(Date.now() + 30 * 24 * 60 * 60 * 1000).toISOString()
      };
    }
    
    // Convertir les noms de champs camelCase vers snake_case pour le backend Go
    const goData = {
      version: "1.0",
      profile: defaultData.profile,
      invoice: {
        number: defaultData.invoice.number,
        issue_date: new Date(defaultData.invoice.issueDate).toISOString(),
        type: defaultData.invoice.typeCode,
        currency: defaultData.invoice.currencyCode
      },
      seller: {
        name: defaultData.seller.name,
        vat_id: defaultData.seller.taxId,
        address: {
          street: defaultData.seller.address.line1,
          city: defaultData.seller.address.city,
          postal_code: defaultData.seller.address.postalCode,
          country: defaultData.seller.address.countryCode
        },
        contact: {
          phone: "+33 1 23 45 67 89",
          email: "contact@entreprise.com"
        },
        global_id: {
          scheme_id: "0009",
          value: "12345678900012"
        },
        bank: defaultData.seller.bank
      },
      buyer: {
        name: defaultData.buyer.name,
        vat_id: defaultData.buyer.taxId,
        address: {
          street: defaultData.buyer.address.line1,
          city: defaultData.buyer.address.city,
          postal_code: defaultData.buyer.address.postalCode,
          country: defaultData.buyer.address.countryCode
        },
        contact: {
          phone: "+33 1 98 76 54 32",
          email: "achat@client.com"
        },
        global_id: {
          scheme_id: "0009",
          value: "98765432100098"
        }
      },
      lines: [
        {
          id: "1",
          description: defaultData.lines[0].description,
          quantity: defaultData.lines[0].quantity,
          unit_price: defaultData.lines[0].unitPrice,
          vat_rate: defaultData.lines[0].vatRate,
          vat_amount: defaultData.lines[0].vatAmount,
          total_excl_vat: defaultData.lines[0].netAmount,
          total_incl_vat: 120.0
        }
      ],
      totals: {
        subtotal_excl_vat: defaultData.totals.netTotal,
        total_vat: defaultData.totals.vatTotal,
        total_incl_vat: defaultData.totals.grossTotal,
        amount_due: defaultData.totals.dueAmount,
        vat_breakdown: [
          {
            rate: 20.0,
            taxable_amount: 100.0,
            vat_amount: 20.0
          }
        ]
      },
      payment: defaultData.payment ? {
        terms: defaultData.payment.terms,
        method: defaultData.payment.method,
        due_date: new Date(defaultData.payment.dueDate).toISOString()
      } : undefined
    };

    return this.http.post(
      `${this.baseUrl}/templates/preview`,
      { template: typstCode, data: invoiceData || goData },
      { responseType: 'blob' },
    );
  }

  // Health check
  health(): Observable<{ status: string }> {
    return this.http.get<{ status: string }>(`${this.baseUrl}/health`);
  }
}
