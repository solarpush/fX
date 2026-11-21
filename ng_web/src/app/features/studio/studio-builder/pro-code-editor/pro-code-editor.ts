import {
  ChangeDetectionStrategy,
  Component,
  ElementRef,
  ViewChild,
  computed,
  effect,
  inject,
  input,
  output,
  signal,
} from '@angular/core';
import { CommonModule } from '@angular/common';
import { DomSanitizer, SafeHtml } from '@angular/platform-browser';
import {
  Diagnostic,
  FX_UI_SNIPPETS,
  TypstSnippetItem,
} from '../../../../core/models/studio.models';

export interface CodeInsertRequest {
  text: string;
  seq: number;
}

interface AutocompleteTrigger {
  kind: 'fx' | 'data' | 'hbs' | 'manual';
  startOffset: number;
  query: string;
}

const SINGLE_PASS_RX =
  /(\/\/[^\n]*|\/\*[\s\S]*?\*\/)|("(?:\\.|[^"\\])*")|(\{\{[^}]+\}\})|(\bfx-[a-zA-Z0-9_-]+\b)|(#(?:import|let|show|set|if|else|for|in|while|return|break|continue|include)\b)|(#[a-zA-Z_][a-zA-Z0-9_-]*)|(\b\d+(?:\.\d+)?(?:pt|mm|cm|in|em|fr|%)\b)|(^[ \t]*={1,6}[^\n]*)/gm;

@Component({
  selector: 'app-pro-code-editor',
  standalone: true,
  changeDetection: ChangeDetectionStrategy.OnPush,
  imports: [CommonModule],
  templateUrl: './pro-code-editor.html',
  styleUrl: './pro-code-editor.scss',
})
export class ProCodeEditorComponent {
  private readonly sanitizer = inject(DomSanitizer);

  readonly code = input.required<string>();
  readonly readonly = input<boolean>(false);
  readonly title = input<string>('template.typ');
  readonly diagnostics = input<readonly Diagnostic[]>([]);
  readonly schemaPaths = input<readonly { path: string; type: string; isArray: boolean }[]>([]);
  readonly insertRequest = input<CodeInsertRequest | null>(null);

  readonly codeChange = output<string>();

  @ViewChild('textareaEl') textareaRef?: ElementRef<HTMLTextAreaElement>;
  @ViewChild('gutterEl') gutterRef?: ElementRef<HTMLDivElement>;
  @ViewChild('highlightEl') highlightRef?: ElementRef<HTMLPreElement>;
  @ViewChild('lineBgEl') lineBgRef?: ElementRef<HTMLDivElement>;

  protected readonly internalCode = signal('');
  protected readonly copied = signal(false);
  protected readonly activeLine = signal(1);
  protected readonly showDiagnosticsPanel = signal(true);

  // Autocomplete state
  protected readonly acOpen = signal(false);
  protected readonly acItems = signal<TypstSnippetItem[]>([]);
  protected readonly acSelectedIdx = signal(0);
  protected readonly acPos = signal<{ top: number; left: number }>({ top: 40, left: 60 });
  private acTrigger: AutocompleteTrigger | null = null;
  private lastInsertSeq = 0;

  constructor() {
    effect(() => {
      const incoming = this.code();
      if (incoming !== this.internalCode()) {
        this.internalCode.set(incoming ?? '');
      }
    });

    effect(() => {
      const req = this.insertRequest();
      if (req && req.seq !== this.lastInsertSeq && !this.readonly()) {
        this.lastInsertSeq = req.seq;
        this.insertAtCursor(req.text);
      }
    });
  }

  protected readonly lineCount = computed(() => {
    const c = this.internalCode();
    return Math.max(1, c.split('\n').length);
  });

  protected readonly linesArray = computed(() =>
    Array.from({ length: this.lineCount() }, (_, i) => i + 1),
  );

  protected readonly diagnosticsByLine = computed(() => {
    const map = new Map<number, Diagnostic[]>();
    for (const d of this.diagnostics()) {
      if (d.line && d.line > 0) {
        const list = map.get(d.line) ?? [];
        list.push(d);
        map.set(d.line, list);
      }
    }
    return map;
  });

  protected readonly errorCount = computed(
    () => this.diagnostics().filter((d) => d.severity !== 'warning').length,
  );

  protected readonly warningCount = computed(
    () => this.diagnostics().filter((d) => d.severity === 'warning').length,
  );

  protected readonly highlightedHtml = computed<SafeHtml>(() => {
    const raw = this.internalCode() || '';
    let result = '';
    let lastIndex = 0;
    SINGLE_PASS_RX.lastIndex = 0;

    let match: RegExpExecArray | null;
    while ((match = SINGLE_PASS_RX.exec(raw)) !== null) {
      if (match.index > lastIndex) {
        result += this.escapeHtml(raw.slice(lastIndex, match.index));
      }
      const token = this.escapeHtml(match[0]);
      if (match[1]) {
        result += `<span style='color:#64748b;font-style:italic'>${token}</span>`;
      } else if (match[2]) {
        result += `<span style='color:#86efac'>${token}</span>`;
      } else if (match[3]) {
        result += `<span style='color:#fbbf24;background:rgba(245,158,11,0.12);border-radius:3px'>${token}</span>`;
      } else if (match[4]) {
        result += `<span style='color:#38bdf8;font-weight:600'>${token}</span>`;
      } else if (match[5]) {
        result += `<span style='color:#c084fc;font-weight:600'>${token}</span>`;
      } else if (match[6]) {
        result += `<span style='color:#818cf8'>${token}</span>`;
      } else if (match[7]) {
        result += `<span style='color:#fb923c'>${token}</span>`;
      } else if (match[8]) {
        result += `<span style='color:#60a5fa;font-weight:700'>${token}</span>`;
      } else {
        result += token;
      }
      lastIndex = match.index + match[0].length;
    }

    if (lastIndex < raw.length) {
      result += this.escapeHtml(raw.slice(lastIndex));
    }
    if (result.endsWith('\n')) {
      result += ' ';
    }
    return this.sanitizer.bypassSecurityTrustHtml(result);
  });

  protected lineSeverity(lineNo: number): 'error' | 'warning' | null {
    const list = this.diagnosticsByLine().get(lineNo);
    if (!list?.length) return null;
    return list.some((d) => d.severity !== 'warning') ? 'error' : 'warning';
  }

  protected lineTooltip(lineNo: number): string {
    const list = this.diagnosticsByLine().get(lineNo);
    if (!list?.length) return '';
    return list.map((d) => `[${d.severity}] ${d.message}`).join('\n');
  }

  protected onInput(event: Event): void {
    const ta = event.target as HTMLTextAreaElement;
    const val = ta.value;
    this.internalCode.set(val);
    this.codeChange.emit(val);
    this.updateCaretLine(ta);
    this.evaluateAutocomplete(ta);
  }

  protected onScroll(event: Event): void {
    const ta = event.target as HTMLTextAreaElement;
    if (this.gutterRef) {
      this.gutterRef.nativeElement.scrollTop = ta.scrollTop;
    }
    if (this.highlightRef) {
      this.highlightRef.nativeElement.scrollTop = ta.scrollTop;
      this.highlightRef.nativeElement.scrollLeft = ta.scrollLeft;
    }
    if (this.lineBgRef) {
      this.lineBgRef.nativeElement.scrollTop = ta.scrollTop;
    }
    if (this.acOpen()) {
      this.positionDropdown(ta);
    }
  }

  protected onClickOrKeyup(event: Event): void {
    const ta = event.target as HTMLTextAreaElement;
    this.updateCaretLine(ta);
  }

  protected onKeyDown(event: KeyboardEvent): void {
    const ta = event.target as HTMLTextAreaElement;

    if (this.acOpen()) {
      const items = this.acItems();
      if (event.key === 'ArrowDown') {
        event.preventDefault();
        this.acSelectedIdx.set((this.acSelectedIdx() + 1) % Math.max(1, items.length));
        return;
      }
      if (event.key === 'ArrowUp') {
        event.preventDefault();
        this.acSelectedIdx.set(
          (this.acSelectedIdx() - 1 + Math.max(1, items.length)) % Math.max(1, items.length),
        );
        return;
      }
      if (event.key === 'Enter' || event.key === 'Tab') {
        const picked = items[this.acSelectedIdx()];
        if (picked) {
          event.preventDefault();
          this.acceptAutocomplete(picked);
          return;
        }
      }
      if (event.key === 'Escape') {
        event.preventDefault();
        this.closeAutocomplete();
        return;
      }
    }

    if (event.ctrlKey && event.code === 'Space' && !this.readonly()) {
      event.preventDefault();
      this.triggerManualAutocomplete(ta);
      return;
    }

    if (event.key === 'Tab' && !this.readonly()) {
      event.preventDefault();
      const start = ta.selectionStart;
      const end = ta.selectionEnd;
      const val = ta.value;
      const next = val.slice(0, start) + '  ' + val.slice(end);
      ta.value = next;
      ta.selectionStart = ta.selectionEnd = start + 2;
      this.internalCode.set(next);
      this.codeChange.emit(next);
    }
  }

  protected acceptAutocomplete(item: TypstSnippetItem): void {
    const ta = this.textareaRef?.nativeElement;
    if (!ta || !this.acTrigger) {
      this.closeAutocomplete();
      return;
    }
    const val = ta.value;
    const end = ta.selectionStart;
    const start = this.acTrigger.startOffset;
    const replacement =
      this.acTrigger.kind === 'hbs' ? `{{${item.insertText}}}` : item.insertText;

    const next = val.slice(0, start) + replacement + val.slice(end);
    ta.value = next;
    const cursor = start + replacement.length;
    ta.selectionStart = ta.selectionEnd = cursor;
    this.internalCode.set(next);
    this.codeChange.emit(next);
    this.closeAutocomplete();
    ta.focus();
  }

  protected closeAutocomplete(): void {
    this.acOpen.set(false);
    this.acTrigger = null;
  }

  protected jumpToLine(line?: number): void {
    if (!line || line <= 0 || !this.textareaRef) return;
    const ta = this.textareaRef.nativeElement;
    const lines = this.internalCode().split('\n');
    let offset = 0;
    for (let i = 0; i < Math.min(line - 1, lines.length); i++) {
      offset += lines[i].length + 1;
    }
    ta.focus();
    ta.setSelectionRange(offset, offset + (lines[line - 1]?.length ?? 0));
    ta.scrollTop = Math.max(0, (line - 4) * 20);
    this.activeLine.set(line);
  }

  protected copyAll(): void {
    if (typeof navigator !== 'undefined' && navigator.clipboard) {
      void navigator.clipboard.writeText(this.internalCode());
      this.copied.set(true);
      setTimeout(() => this.copied.set(false), 1400);
    }
  }

  private insertAtCursor(text: string): void {
    const ta = this.textareaRef?.nativeElement;
    const curr = this.internalCode();
    if (!ta) {
      const next = curr + (curr.endsWith('\n') ? '' : '\n') + text;
      this.internalCode.set(next);
      this.codeChange.emit(next);
      return;
    }
    const start = ta.selectionStart ?? curr.length;
    const end = ta.selectionEnd ?? curr.length;
    const next = curr.slice(0, start) + text + curr.slice(end);
    ta.value = next;
    ta.selectionStart = ta.selectionEnd = start + text.length;
    this.internalCode.set(next);
    this.codeChange.emit(next);
    ta.focus();
  }

  private updateCaretLine(ta: HTMLTextAreaElement): void {
    const pos = ta.selectionStart ?? 0;
    const line = ta.value.slice(0, pos).split('\n').length;
    this.activeLine.set(line);
  }

  private evaluateAutocomplete(ta: HTMLTextAreaElement): void {
    if (this.readonly()) {
      this.closeAutocomplete();
      return;
    }
    const pos = ta.selectionStart;
    const before = ta.value.slice(Math.max(0, pos - 60), pos);

    // 1. Trigger on `fx-`
    const fxMatch = before.match(/(fx-[a-zA-Z0-9_-]*)$/);
    if (fxMatch) {
      const q = fxMatch[1].toLowerCase();
      const matches = FX_UI_SNIPPETS.filter((s) => s.label.toLowerCase().includes(q));
      if (matches.length > 0) {
        this.openAutocomplete(
          { kind: 'fx', startOffset: pos - fxMatch[1].length, query: q },
          matches,
          ta,
        );
        return;
      }
    }

    // 2. Trigger on `data.`
    const dataMatch = before.match(/(data\.[a-zA-Z0-9_.]*)$/);
    if (dataMatch) {
      const prefix = dataMatch[1].slice(5).toLowerCase();
      const matches = this.buildSchemaSnippets('data', prefix);
      if (matches.length > 0) {
        this.openAutocomplete(
          { kind: 'data', startOffset: pos - dataMatch[1].length, query: prefix },
          matches,
          ta,
        );
        return;
      }
    }

    // 3. Trigger on `{{`
    const hbsMatch = before.match(/(\{\{[a-zA-Z0-9_.]*)$/);
    if (hbsMatch) {
      const prefix = hbsMatch[1].slice(2).toLowerCase();
      const matches = this.buildSchemaSnippets('hbs', prefix);
      if (matches.length > 0) {
        this.openAutocomplete(
          { kind: 'hbs', startOffset: pos - hbsMatch[1].length, query: prefix },
          matches,
          ta,
        );
        return;
      }
    }

    this.closeAutocomplete();
  }

  private triggerManualAutocomplete(ta: HTMLTextAreaElement): void {
    const pos = ta.selectionStart;
    const items: TypstSnippetItem[] = [
      ...FX_UI_SNIPPETS,
      ...this.buildSchemaSnippets('data', ''),
    ].slice(0, 24);
    if (items.length > 0) {
      this.openAutocomplete({ kind: 'manual', startOffset: pos, query: '' }, items, ta);
    }
  }

  private buildSchemaSnippets(mode: 'data' | 'hbs', filter: string): TypstSnippetItem[] {
    return this.schemaPaths()
      .map((sp) => {
        const clean = sp.path.replace(/\[\]\./g, '.').replace(/\[\]$/, '');
        return {
          label: mode === 'data' ? `data.${clean}` : `{{${clean}}}`,
          kind: 'data' as const,
          detail: sp.isArray ? 'tableau JSON' : `champ schéma (${sp.type})`,
          insertText:
            mode === 'data'
              ? sp.isArray
                ? `fx-list(data, "${clean}")`
                : `fx-get(data, "${clean}")`
              : clean,
        };
      })
      .filter((it) => !filter || it.label.toLowerCase().includes(filter))
      .slice(0, 16);
  }

  private openAutocomplete(
    trigger: AutocompleteTrigger,
    items: TypstSnippetItem[],
    ta: HTMLTextAreaElement,
  ): void {
    this.acTrigger = trigger;
    this.acItems.set(items.slice(0, 12));
    this.acSelectedIdx.set(0);
    this.acOpen.set(true);
    this.positionDropdown(ta);
  }

  private positionDropdown(ta: HTMLTextAreaElement): void {
    const before = ta.value.slice(0, ta.selectionStart);
    const lines = before.split('\n');
    const lineIdx = lines.length - 1;
    const col = lines[lines.length - 1]?.length ?? 0;

    const top = Math.max(24, Math.min(ta.clientHeight - 160, (lineIdx + 1) * 20 - ta.scrollTop + 14));
    const left = Math.max(54, Math.min(ta.clientWidth - 280, 54 + col * 7.6 - ta.scrollLeft));
    this.acPos.set({ top, left });
  }

  private escapeHtml(str: string): string {
    return str.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;');
  }
}
