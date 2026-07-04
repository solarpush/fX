import { CommonModule } from '@angular/common';
import { HttpClient } from '@angular/common/http';
import { Component, inject, OnDestroy, OnInit, signal } from '@angular/core';
import { FormsModule } from '@angular/forms';
import { ActivatedRoute, Router, RouterLink } from '@angular/router';
import { environment } from '../../../../environments/environment';
import { Template } from '../../../core/models';
import { TemplateStorage } from '../../../core/services/template-storage';
import { TypstCompiler } from '../../../core/services/typst-compiler';
import { CodeEditor } from './code-editor/code-editor';
import { LivePreview } from './live-preview/live-preview';

@Component({
  selector: 'app-template-editor',
  imports: [CommonModule, FormsModule, CodeEditor, LivePreview, RouterLink],
  templateUrl: './template-editor.html',
  styleUrl: './template-editor.scss',
})
export class TemplateEditor implements OnInit, OnDestroy {
  private readonly route = inject(ActivatedRoute);
  private readonly router = inject(Router);
  private readonly templateStorage = inject(TemplateStorage);
  private readonly compiler = inject(TypstCompiler);
  private readonly http = inject(HttpClient);

  protected readonly template = signal<Template | null>(null);
  protected readonly typstCode = signal('');
  protected readonly targetProfile = signal<string>('EN16931');
  protected readonly aiPrompt = signal('');
  protected readonly isGenerating = signal(false);
  protected readonly previewBlob = signal<Blob | null>(null);
  protected readonly isCompiling = signal(false);
  protected readonly isSaving = signal(false);

  private compileTimeout: any;

  protected updateProfile(profile: string) {
    this.targetProfile.set(profile);
    this.syncMetadataToCode();
  }

  private syncMetadataToCode() {
    let code = this.typstCode();

    // Update profile
    if (/\/\/\s*@profile:.*/.test(code)) {
      code = code.replace(/\/\/\s*@profile:.*/, `// @profile: ${this.targetProfile()}`);
    } else {
      code = `// @profile: ${this.targetProfile()}\n` + code;
    }

    this.typstCode.set(code);
  }

  ngOnInit(): void {
    this.route.paramMap.subscribe(async (params) => {
      const templateId = params.get('id');

      if (templateId) {
        try {
          const template = await this.templateStorage.loadTemplate(templateId);
          this.template.set(template);
          this.onCodeChange(template.content || '');
          this.compilePreview();
        } catch (error) {
          console.error('Failed to load template:', error);
        }
      } else {
        // Nouveau template
        this.template.set(null);
        this.onCodeChange(this.getDefaultTemplate());
        this.compilePreview();
      }
    });
  }

  ngOnDestroy(): void {
    if (this.compileTimeout) {
      clearTimeout(this.compileTimeout);
    }
  }

  protected onCodeChange(code: string): void {
    this.typstCode.set(code);

    // Auto-détection du profil et capacités depuis le code
    const profileMatch = code.match(/\/\/\s*@profile:\s*([A-Z0-9_]+)/);
    if (profileMatch) {
      this.targetProfile.set(profileMatch[1]);
    }


    // Debounce compilation
    if (this.compileTimeout) {
      clearTimeout(this.compileTimeout);
    }
    this.compileTimeout = setTimeout(() => {
      this.compilePreview();
    }, 500);
  }

  protected readonly compilationError = signal<string | null>(null);



  private async compilePreview(): Promise<void> {
    this.isCompiling.set(true);
    let rules: any = null;

    try {
      rules = await this.compiler.getRules(this.targetProfile()).toPromise();
      const code = this.typstCode();
      const missing = rules?.required_tags.filter((tag: string) => !code.includes(tag)) || [];
      
      if (missing.length > 0) {
        this.compilationError.set(
          `Non conforme au profil ${this.targetProfile()}. Il manque les variables suivantes :\n- ${missing.map((m: string) => '{{' + m + '}}').join('\n- ')}`,
        );
      } else {
        this.compilationError.set('');
      }
    } catch (e) {
      console.error('Failed to get template rules:', e);
      this.compilationError.set('');
    }

    try {
      const blob = await this.compiler.compile(this.typstCode(), this.targetProfile(), rules?.mock_data).toPromise();
      if (blob) {
        this.previewBlob.set(blob);
      }
    } catch (error: any) {
      console.error('Compilation failed:', error);
      if (error.error instanceof Blob) {
        try {
          const text = await error.error.text();
          const json = JSON.parse(text);
          this.compilationError.set(json.error || 'Erreur de compilation Typst');
        } catch {
          this.compilationError.set('Erreur de compilation Typst');
        }
      } else {
        this.compilationError.set('Erreur de compilation Typst');
      }
    } finally {
      this.isCompiling.set(false);
    }
  }

  protected readonly showSaveModal = signal(false);
  protected readonly newTemplateName = signal('');

  protected saveTemplateAs(): void {
    this.newTemplateName.set(this.template()?.name || 'Nouveau template');
    this.showSaveModal.set(true);
  }

  protected async confirmSaveAs(): Promise<void> {
    if (!this.newTemplateName().trim()) return;
    this.showSaveModal.set(false);
    this.isSaving.set(true);

    try {
      const template = this.template();
      const data: Partial<Template> = {
        name: this.newTemplateName().trim(),
        content: this.typstCode(),
        category: template?.category,
        tags: template?.tags || [],
        isDefault: false,
      };

      const created = await this.templateStorage.createTemplate(data);
      // Mettre à jour l'état local pour qu'on édite maintenant le nouveau fichier
      this.template.set(created);
      this.router.navigate(['/templates', created.id]);
    } catch (error) {
      console.error('Failed to save template as:', error);
    } finally {
      this.isSaving.set(false);
    }
  }

  protected async saveTemplate(): Promise<void> {
    const template = this.template();
    // Si c'est un nouveau template sans ID, on force le "Enregistrer sous"
    if (!template?.id) {
      this.saveTemplateAs();
      return;
    }

    this.isSaving.set(true);
    try {
      const data: Partial<Template> = {
        name: template.name,
        content: this.typstCode(),
        category: template.category,
        tags: template.tags || [],
        isDefault: false,
      };

      await this.templateStorage.updateTemplate(template.id, data);
    } catch (error) {
      console.error('Failed to save template:', error);
    } finally {
      this.isSaving.set(false);
    }
  }

  private getDefaultTemplate(): string {
    return `// @profile: EN16931

#set page(
  paper: "a4",
  margin: (x: 2cm, y: 2.5cm),
  header: align(right)[
    #text(size: 8.5pt, fill: rgb("#94a3b8"))[Document généré automatiquement - Factur-X Compliant]
  ],
  footer: [
    #place(top, line(length: 100%, stroke: 0.5pt + rgb("#e2e8f0")))
    #v(0.5em)
    #grid(
      columns: (1fr, 1fr),
      text(size: 8pt, fill: rgb("#64748b"))[
        #strong[{{seller.name}}] \
        {{seller.address.street}}, {{seller.address.postal_code}} {{seller.address.city}}, {{seller.address.country}} \
        #if "{{seller.vat_id}}" != "" [TVA : {{seller.vat_id}}]
      ],
      align(right + bottom)[
        #text(size: 8pt, fill: rgb("#64748b"))[Page #context counter(page).display("1", both: false)]
      ]
    )
  ]
)

#set text(
  font: "Liberation Sans",
  size: 10pt,
  fill: rgb("#1e293b")
)

#grid(
  columns: (1fr, 1fr),
  gutter: 1cm,
  [
    #text(size: 18pt, weight: "bold", fill: rgb("#0f172a"))[{{seller.name}}]
    #v(0.8em)
    #text(size: 9.5pt, fill: rgb("#334155"))[
      {{seller.address.street}} \
      {{seller.address.postal_code}} {{seller.address.city}} \
      {{seller.address.country}} \
      #if "{{seller.contact.phone}}" != "" [Tél : {{seller.contact.phone}} \ ]
      #if "{{seller.contact.email}}" != "" [Mél : {{seller.contact.email}}]
    ]
  ],
  align(right)[
    #text(size: 20pt, weight: "light", fill: rgb("#0f172a"))[
      #if "{{invoice.type}}" == "381" [AVOIR] else if "{{invoice.type}}" == "384" [FACTURE RECTIFICATIVE] else [FACTURE]
    ] \
    #text(size: 12pt, weight: "bold", fill: rgb("#64748b"))[N° {{invoice.number}}]
    
    #v(1.5em)
    #grid(
      columns: (auto, auto),
      gutter: 0.6em,
      align: (right, left),
      [Date d'émission :], [#strong[{{invoice.issue_date}}]],
      [Date d'échéance :], [#strong[{{invoice.due_date}}]],
      ..if "{{invoice.purchase_order_ref}}" != "" {
        ([Réf. Commande :], [#strong[{{invoice.purchase_order_ref}}]])
      } else {
        ()
      }
    )
  ]
)

#v(2.5cm)

#grid(
  columns: (3fr, 2fr),
  gutter: 1cm,
  [
    #text(size: 8.5pt, weight: "bold", fill: rgb("#94a3b8"))[FACTURÉ À]
    #v(0.5em)
    #text(size: 11pt, weight: "bold", fill: rgb("#0f172a"))[{{buyer.name}}]
    #v(0.3em)
    #text(size: 9.5pt, fill: rgb("#334155"))[
      {{buyer.address.street}} \
      {{buyer.address.postal_code}} {{buyer.address.city}} \
      {{buyer.address.country}} \
      #if "{{buyer.vat_id}}" != "" [N° TVA : {{buyer.vat_id}}]
    ]
  ],
  []
)

#v(2cm)

#table(
  columns: (1fr, auto, auto, auto, auto),
  align: (left, right, center, right, right),
  stroke: (x, y) => if y == 0 { (bottom: 1.5pt + rgb("#0f172a")) } else { (bottom: 0.5pt + rgb("#e2e8f0")) },
  fill: (x, y) => if y == 0 { rgb("#f8fafc") } else { none },
  inset: 0.8em,
  [#strong[Désignation]], [#strong[Qté]], [#strong[Unité]], [#strong[Prix unitaire]], [#strong[TVA]],
  {{#each lines}}
  [{{description}}], [{{quantity}}], [{{unit}}], [{{unit_price}} {{invoice.currency}}], [{{vat_rate}}%],
  {{/each}}
)

#v(1cm)

#grid(
  columns: (1fr, 180pt),
  gutter: 20pt,
  [
    #text(size: 8.5pt, weight: "bold", fill: rgb("#94a3b8"))[DÉTAIL DE LA TVA]
    #v(0.5em)
    #table(
      columns: (1fr, 1.2fr, 1.2fr),
      align: (left, right, right),
      stroke: (x, y) => if y == 0 { (bottom: 1.5pt + rgb("#0f172a")) } else { (bottom: 0.5pt + rgb("#e2e8f0")) },
      fill: (x, y) => if y == 0 { rgb("#f8fafc") } else { none },
      inset: 0.6em,
      [*Taux*], [*Base HT*], [*Montant TVA*],
      {{#each totals.vat_breakdown}}
      [{{rate}}%], [{{taxable_amount}} {{invoice.currency}}], [{{vat_amount}} {{invoice.currency}}],
      {{/each}}
    )
    
    #v(1em)
    #if "{{invoice.note}}" != "" [
      #text(size: 8.5pt, weight: "bold", fill: rgb("#94a3b8"))[NOTE] \
      #v(0.3em)
      #text(size: 8.5pt, fill: rgb("#475569"))[{{invoice.note}}]
    ]
  ],
  [
    #set text(size: 9.5pt)
    #grid(
      columns: (1fr, auto),
      row-gutter: 0.8em,
      align: (left, right),
      [Montant HT :], [{{totals.subtotal_excl_vat}} {{invoice.currency}}],
      [Montant TVA :], [{{totals.total_vat}} {{invoice.currency}}],
      line(length: 100%, stroke: 0.5pt + rgb("#cbd5e0")), line(length: 100%, stroke: 0.5pt + rgb("#cbd5e0")),
      [#strong[Total TTC :]], [#strong[{{totals.total_incl_vat}} {{invoice.currency}}]],
      ..if "{{totals.amount_due}}" != "{{totals.total_incl_vat}}" {
        ([#strong[Reste à payer :]], [#strong[{{totals.amount_due}} {{invoice.currency}}]])
      } else {
        ()
      }
    )
  ]
)

#v(2cm)

#grid(
  columns: (1fr, 1fr),
  gutter: 1.5cm,
  [
    #text(size: 8.5pt, weight: "bold", fill: rgb("#94a3b8"))[INFORMATIONS DE PAIEMENT]
    #v(0.5em)
    #text(size: 8.5pt, fill: rgb("#475569"))[
      #if "{{payment.method}}" != "" [Mode de règlement : {{payment.method}} \ ]
      #if "{{payment.terms}}" != "" [Conditions : {{payment.terms}}]
    ]
  ],
  [
    #if "{{seller.bank.iban}}" != "" [
      #text(size: 8.5pt, weight: "bold", fill: rgb("#94a3b8"))[COORDONNÉES BANCAIRES]
      #v(0.5em)
      #text(size: 8.5pt, fill: rgb("#475569"))[
        IBAN : {{seller.bank.iban}} \
        #if "{{seller.bank.bic}}" != "" [BIC : {{seller.bank.bic}}]
      ]
    ]
  ]
)
`;
  }

  protected async generateWithAI(): Promise<void> {
    if (!this.aiPrompt().trim()) return;

    this.isGenerating.set(true);
    try {
      let dataSchema = '';
      try {
        const rules = await this.compiler.getRules(this.targetProfile()).toPromise();
        dataSchema = "Variables Factur-X disponibles pour injection (syntaxe Handlebars ex: {{invoice.number}}) :\n\n" + (rules?.ai_prompt || '');
      } catch (e) {
        console.error('Failed to get template rules for AI:', e);
      }

      const req = {
        prompt: this.aiPrompt(),
        current_typst: this.typstCode(),
        target_profile: this.targetProfile(),
        data_schema: dataSchema,
      };

      const res = await this.http
        .post<{ data: { typst_code: string } }>(`${environment.apiUrl}/ai/generate`, req)
        .toPromise();

      if (res?.data?.typst_code) {
        let code = res.data.typst_code;

        // Nettoyer toute tentative de l'IA d'ajouter les métadonnées (parfois elle les formate mal)
        code = code.replace(/\/\/\s*@profile:.*(\n|$)/gi, '');
        code = code.replace(/\/\/\s*@capabilities:.*(\n|$)/gi, '');

        // Injecter le profil cible proprement depuis l'état de l'UI
        const header = `// @profile: ${this.targetProfile()}\n\n`;
        code = header + code.trim();

        this.typstCode.set(code);
        this.onCodeChange(code);
        this.aiPrompt.set('');
      }
    } catch (error) {
      console.error('AI generation failed:', error);
      alert('La génération IA a échoué. Vérifiez vos clés API ou les logs.');
    } finally {
      this.isGenerating.set(false);
    }
  }

  protected fixErrorWithAI(): void {
    const errorMsg = this.compilationError();
    if (!errorMsg) return;

    if (errorMsg.includes('Non conforme au profil')) {
      this.aiPrompt.set(
        `Le template ne respecte pas les exigences du profil.\nVoici le diagnostic :\n${errorMsg}\n\nAjoute les variables manquantes dans le code de manière esthétique.`,
      );
    } else if (errorMsg.includes('already in code mode')) {
      this.aiPrompt.set(
        `Le code actuel ne compile pas. L'erreur indique que tu as utilisé le caractère '#' alors que le contexte était déjà en mode code (souvent à l'intérieur d'une fonction comme #align, #table ou #rect).\nVoici l'erreur :\n${errorMsg}\n\nRetire le '#' excédentaire (ex: utilise 'rect()' au lieu de '#rect()') pour que ça compile.`,
      );
    } else {
      this.aiPrompt.set(
        `Le code actuel ne compile pas. Voici l'erreur : \n${errorMsg}\n\nCorrige le code pour qu'il compile.`,
      );
    }
    this.generateWithAI();
  }
}
