import { Injectable, inject } from '@angular/core';
import { Observable } from 'rxjs';
import { Api } from './api';

@Injectable({
  providedIn: 'root',
})
export class TypstCompiler {
  private readonly api = inject(Api);

  compile(typstCode: string, profile: string = 'EN16931'): Observable<Blob> {
    return this.api.compilePreview(typstCode, profile);
  }
}
