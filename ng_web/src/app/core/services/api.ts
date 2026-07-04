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

// --- Custom templates (feature-flagged) ---
export interface CustomTemplate {
  id: string;
  name: string;
  type?: string;
  content?: string;
  schema?: string;
  updatedAt?: string;
}

interface CustomTemplateListData {
  templates: CustomTemplate[];
  total: number;
}

export interface CustomValidationResult {
  valid: boolean;
  schemaErrors: string[];
  missingInTemplate: string[];
  unknownInTemplate: string[];
  mock: unknown;
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

  getTemplateRules(profile: string): Observable<{ required_tags: string[]; optional_tags: string[]; ai_prompt: string; mock_data: any }> {
    return this.http
      .get<ApiResponse<{ required_tags: string[]; optional_tags: string[]; ai_prompt: string; mock_data: any }>>(`${this.baseUrl}/templates/rules?profile=${profile}`)
      .pipe(map((res) => res.data));
  }

  compilePreview(typstCode: string, profile: string = 'EN16931', invoiceData?: any): Observable<Blob> {
    const req = {
      template: typstCode,
      data: invoiceData // Données mock injectées depuis le backend
    };

    return this.http.post<Blob>(`${this.baseUrl}/templates/preview`, req, {
      responseType: 'blob' as 'json',
    });
  }

  // Health check
  health(): Observable<{ status: string }> {
    return this.http.get<{ status: string }>(`${this.baseUrl}/health`);
  }

  // ============================================================
  // Custom templates (feature-flag ALLOW_CUSTOM_TEMPLATES)
  // Scope isolé : template Typst libre + JSON Schema.
  // ============================================================

  listCustomTemplates(): Observable<CustomTemplate[]> {
    return this.http
      .get<ApiResponse<CustomTemplateListData>>(`${this.baseUrl}/custom/templates`)
      .pipe(map((res) => res.data?.templates ?? []));
  }

  getCustomTemplate(id: string): Observable<CustomTemplate> {
    return this.http
      .get<ApiResponse<CustomTemplate>>(`${this.baseUrl}/custom/templates/${encodeURIComponent(id)}`)
      .pipe(map((res) => res.data));
  }

  createCustomTemplate(payload: {
    name: string;
    content: string;
    schema: string;
  }): Observable<CustomTemplate> {
    return this.http
      .post<ApiResponse<CustomTemplate>>(`${this.baseUrl}/custom/templates`, payload)
      .pipe(map((res) => res.data));
  }

  updateCustomTemplate(
    id: string,
    payload: { content: string; schema: string },
  ): Observable<CustomTemplate> {
    return this.http
      .put<ApiResponse<CustomTemplate>>(
        `${this.baseUrl}/custom/templates/${encodeURIComponent(id)}`,
        payload,
      )
      .pipe(map((res) => res.data));
  }

  deleteCustomTemplate(id: string): Observable<void> {
    return this.http.delete<void>(`${this.baseUrl}/custom/templates/${encodeURIComponent(id)}`);
  }

  customValidate(
    template: string,
    schema: string,
    data?: unknown,
  ): Observable<CustomValidationResult> {
    return this.http
      .post<ApiResponse<CustomValidationResult>>(`${this.baseUrl}/custom/validate`, {
        template,
        schema: schema ? JSON.parse(schema) : undefined,
        data,
      })
      .pipe(map((res) => res.data));
  }

  customPreview(template: string, schema: string, data?: unknown): Observable<Blob> {
    return this.http.post(
      `${this.baseUrl}/custom/preview`,
      { template, schema: schema ? JSON.parse(schema) : undefined, data },
      { responseType: 'blob' },
    );
  }

  customAiGenerate(payload: {
    prompt: string;
    current_typst: string;
    schema: string;
  }): Observable<string> {
    return this.http
      .post<ApiResponse<{ typst_code: string }>>(`${this.baseUrl}/custom/ai/generate`, payload)
      .pipe(map((res) => res.data?.typst_code ?? ''));
  }
}
