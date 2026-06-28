import { Component, effect, input, output, signal, ElementRef, ViewChild, computed, inject } from '@angular/core';
import { FormsModule } from '@angular/forms';
import { DomSanitizer, SafeHtml } from '@angular/platform-browser';

@Component({
  selector: 'app-code-editor',
  imports: [FormsModule],
  templateUrl: './code-editor.html',
  styleUrl: './code-editor.scss',
})
export class CodeEditor {
  private readonly sanitizer = inject(DomSanitizer);

  readonly code = input.required<string>();
  readonly codeChange = output<string>();

  @ViewChild('lineNumbers') lineNumbersRef!: ElementRef<HTMLDivElement>;
  @ViewChild('highlightLayer') highlightLayerRef!: ElementRef<HTMLPreElement>;

  protected readonly internalCode = signal('');

  protected readonly lines = computed(() => {
    const code = this.internalCode() || '';
    return Array.from({ length: Math.max(1, code.split('\n').length) });
  });

  protected readonly highlightedCode = computed<SafeHtml>(() => {
    let code = this.internalCode() || '';
    
    // Escape HTML to prevent injection and rendering issues
    code = code.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;');
    
    // Apply basic syntax highlighting using regex with inline styles to bypass ViewEncapsulation
    // NOTE: Using single quotes for HTML attributes to prevent the string regex from matching them!
    
    // 1. Comments
    code = code.replace(/(\/\/.*)/g, "<span class='hl-comment'>$1</span>");
    // 2. Typst #commands (avoid matching inside HTML attributes by checking word boundary or simply using class)
    code = code.replace(/(?<!&)(#[a-zA-Z][a-zA-Z0-9_-]*)/g, "<span class='hl-command'>$1</span>");
    // 3. Handlebars
    code = code.replace(/(\{\{[^}]+\}\})/g, "<span class='hl-hbs'>$1</span>");
    // 4. Strings
    code = code.replace(/(&quot;[^&quot;]*&quot;|"[^"]*")/g, "<span class='hl-string'>$1</span>");
    // 5. Headers
    code = code.replace(/^(={1,6}.*)$/gm, "<span class='hl-header'>$1</span>");
    
    // Replace classes with styles at the end to avoid matching our own hex codes!
    code = code.replace(/class='hl-comment'/g, "style='color: #64748b;'");
    code = code.replace(/class='hl-command'/g, "style='color: #c678dd;'");
    code = code.replace(/class='hl-hbs'/g, "style='color: #e5c07b;'");
    code = code.replace(/class='hl-string'/g, "style='color: #98c379;'");
    code = code.replace(/class='hl-header'/g, "style='color: #61afef;'");
    
    // Add trailing space for correct empty line rendering
    if (code.endsWith('\n')) {
      code += ' ';
    }
    
    return this.sanitizer.bypassSecurityTrustHtml(code);
  });

  constructor() {
    effect(() => {
      this.internalCode.set(this.code());
    });
  }

  protected onInput(event: Event): void {
    const value = (event.target as HTMLTextAreaElement).value;
    this.internalCode.set(value);
    this.codeChange.emit(value);
  }

  protected onScroll(event: Event): void {
    const target = event.target as HTMLTextAreaElement;
    if (this.lineNumbersRef) {
      this.lineNumbersRef.nativeElement.scrollTop = target.scrollTop;
    }
    if (this.highlightLayerRef) {
      this.highlightLayerRef.nativeElement.scrollTop = target.scrollTop;
      this.highlightLayerRef.nativeElement.scrollLeft = target.scrollLeft;
    }
  }

  protected onKeyDown(event: KeyboardEvent): void {
    const textarea = event.target as HTMLTextAreaElement;

    // Tab key support
    if (event.key === 'Tab') {
      event.preventDefault();
      const start = textarea.selectionStart;
      const end = textarea.selectionEnd;
      const value = textarea.value;

      textarea.value = value.substring(0, start) + '  ' + value.substring(end);
      textarea.selectionStart = textarea.selectionEnd = start + 2;
      this.onInput(event);
    }
  }
}
