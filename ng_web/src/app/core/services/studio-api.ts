import { HttpClient, HttpHeaders, HttpParams } from '@angular/common/http';
import { Injectable, inject } from '@angular/core';
import { Observable, map } from 'rxjs';
import { environment } from '../../../environments/environment';
import {
  Blueprint,
  BlueprintCompileResponse,
  BlueprintPatch,
  BlueprintPatchResponse,
  CapabilitiesData,
  OutputFormat,
  RenderRequest,
  RenderResponse,
  SaveStudioTemplateRequest,
  SchemaGenerateRequest,
  SchemaGenerateResult,
  SchemaInferResponse,
  StudioGenerateRequest,
  StudioGenerateResult,
  StudioPresetsData,
  StudioTemplateRecord,
  StudioTemplatesListData,
  StudioVersionsListData,
} from '../models/studio.models';

interface ApiEnvelope<T> {
  success?: boolean;
  data?: T;
  error?: string;
}

@Injectable({
  providedIn: 'root',
})
export class StudioApiService {
  private readonly http = inject(HttpClient);
  private readonly baseUrl = environment.apiUrl;

  private unwrap<T>(res: ApiEnvelope<T> | T): T {
    if (res && typeof res === 'object' && 'data' in res && (res as ApiEnvelope<T>).data !== undefined) {
      return (res as ApiEnvelope<T>).data as T;
    }
    return res as T;
  }

  getCapabilities(): Observable<CapabilitiesData> {
    return this.http
      .get<ApiEnvelope<CapabilitiesData>>(`${this.baseUrl}/capabilities`)
      .pipe(map((res) => this.unwrap(res)));
  }

  getPresets(): Observable<StudioPresetsData> {
    return this.http
      .get<ApiEnvelope<StudioPresetsData>>(`${this.baseUrl}/studio/presets`)
      .pipe(map((res) => this.unwrap(res)));
  }

  listTemplates(kind?: string): Observable<StudioTemplatesListData> {
    let params = new HttpParams();
    if (kind && kind !== 'all') {
      params = params.set('kind', kind);
    }
    return this.http
      .get<ApiEnvelope<StudioTemplatesListData>>(`${this.baseUrl}/studio/templates`, { params })
      .pipe(map((res) => this.unwrap(res)));
  }

  getTemplate(id: string): Observable<StudioTemplateRecord> {
    return this.http
      .get<ApiEnvelope<StudioTemplateRecord>>(
        `${this.baseUrl}/studio/templates/${encodeURIComponent(id)}`,
      )
      .pipe(map((res) => this.unwrap(res)));
  }

  createTemplate(payload: SaveStudioTemplateRequest): Observable<StudioTemplateRecord> {
    return this.http
      .post<ApiEnvelope<StudioTemplateRecord>>(`${this.baseUrl}/studio/templates`, payload)
      .pipe(map((res) => this.unwrap(res)));
  }

  updateTemplate(id: string, payload: SaveStudioTemplateRequest): Observable<StudioTemplateRecord> {
    return this.http
      .put<ApiEnvelope<StudioTemplateRecord>>(
        `${this.baseUrl}/studio/templates/${encodeURIComponent(id)}`,
        payload,
      )
      .pipe(map((res) => this.unwrap(res)));
  }

  deleteTemplate(id: string): Observable<{ message: string }> {
    return this.http
      .delete<ApiEnvelope<{ message: string }>>(
        `${this.baseUrl}/studio/templates/${encodeURIComponent(id)}`,
      )
      .pipe(map((res) => this.unwrap(res)));
  }

  listVersions(id: string): Observable<StudioVersionsListData> {
    return this.http
      .get<ApiEnvelope<StudioVersionsListData>>(
        `${this.baseUrl}/studio/templates/${encodeURIComponent(id)}/versions`,
      )
      .pipe(map((res) => this.unwrap(res)));
  }

  compileBlueprint(blueprint: Blueprint, schema?: string): Observable<BlueprintCompileResponse> {
    return this.http
      .post<ApiEnvelope<BlueprintCompileResponse>>(`${this.baseUrl}/studio/blueprint/compile`, {
        blueprint,
        schema: schema || undefined,
      })
      .pipe(map((res) => this.unwrap(res)));
  }

  patchBlueprint(
    blueprint: Blueprint,
    patches: BlueprintPatch[],
  ): Observable<BlueprintPatchResponse> {
    return this.http
      .post<ApiEnvelope<BlueprintPatchResponse>>(`${this.baseUrl}/studio/blueprint/patch`, {
        blueprint,
        patches,
      })
      .pipe(map((res) => this.unwrap(res)));
  }

  inferSchema(sampleData: unknown): Observable<SchemaInferResponse> {
    return this.http
      .post<ApiEnvelope<SchemaInferResponse>>(`${this.baseUrl}/studio/schema/infer`, {
        sampleData,
      })
      .pipe(map((res) => this.unwrap(res)));
  }

  renderJson(req: RenderRequest): Observable<RenderResponse> {
    return this.http
      .post<ApiEnvelope<RenderResponse>>(`${this.baseUrl}/render`, req)
      .pipe(map((res) => this.unwrap(res)));
  }

  renderBinary(req: RenderRequest, format: OutputFormat = 'pdf'): Observable<Blob> {
    const accept =
      format === 'png'
        ? 'image/png'
        : format === 'svg'
          ? 'image/svg+xml'
          : 'application/pdf';
    const headers = new HttpHeaders({ Accept: accept });
    return this.http.post(`${this.baseUrl}/render`, { ...req, format }, {
      headers,
      responseType: 'blob',
    });
  }

  aiGenerateStudio(req: StudioGenerateRequest): Observable<StudioGenerateResult> {
    return this.http
      .post<ApiEnvelope<StudioGenerateResult>>(`${this.baseUrl}/ai/studio/generate`, req)
      .pipe(map((res) => this.unwrap(res)));
  }

  aiGenerateSchema(req: SchemaGenerateRequest): Observable<SchemaGenerateResult> {
    return this.http
      .post<ApiEnvelope<SchemaGenerateResult>>(`${this.baseUrl}/ai/schema/generate`, req)
      .pipe(map((res) => this.unwrap(res)));
  }
}
