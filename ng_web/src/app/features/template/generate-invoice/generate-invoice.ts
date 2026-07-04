import { CommonModule } from '@angular/common';
import { ChangeDetectorRef, Component, inject, OnInit, signal } from '@angular/core';
import { FormArray, FormBuilder, FormGroup, ReactiveFormsModule, Validators } from '@angular/forms';
import { ActivatedRoute, Router, RouterLink } from '@angular/router';
import { Invoice } from '../../../core/models';
import { Api } from '../../../core/services/api';

@Component({
  selector: 'app-generate-invoice',
  standalone: true,
  imports: [CommonModule, ReactiveFormsModule, RouterLink],
  templateUrl: './generate-invoice.html',
  styleUrl: './generate-invoice.css',
})
export class GenerateInvoice implements OnInit {
  private readonly fb = inject(FormBuilder);
  private readonly route = inject(ActivatedRoute);
  private readonly router = inject(Router);
  private readonly api = inject(Api);
  private readonly cdr = inject(ChangeDetectorRef);

  protected templateId = signal<string>('');
  protected targetProfile = signal<string>('EN16931');
  protected capabilities = signal<string[]>([]);

  protected invoiceForm!: FormGroup;
  protected isGenerating = signal<boolean>(false);
  protected error = signal<string | null>(null);
  protected validationErrors = signal<string[]>([]);

  ngOnInit(): void {
    const id = this.route.snapshot.paramMap.get('id');
      
    if (!id) {
      this.router.navigate(['/templates']);
      return;
    }
    this.templateId.set(id);

    this.api.getTemplate(id).subscribe({
      next: (tpl) => {
        
        if (tpl.content) {
          const profileMatch = tpl.content.match(/\/\/\s*@profile:\s*([A-Z0-9_]+)/);
          if (profileMatch) this.targetProfile.set(profileMatch[1]);

          const capMatch = tpl.content.match(/\/\/\s*@capabilities:\s*(.*)/);
          if (capMatch) {
            this.capabilities.set(
              capMatch[1]
                .split(',')
                .map((c) => c.trim())
                .filter((c) => c.length > 0),
            );
          }
        }
        this.initForm();
        
      },
      error: (err) => {
        console.error('Error loading template:', err);
        this.error.set('Impossible de charger le template.');
      },
    });
  }

  private initForm(): void {
    const p = this.targetProfile();
    const c = this.capabilities();

    console.log('initForm called', { profile: p, capabilities: c });

    // Determine requirements
    const requireBankInfo = p === 'EXTENDED' || c.includes('bank_info');
    const requirePaymentTerms = p === 'EXTENDED' || c.includes('payment_terms');

    this.invoiceForm = this.fb.group({
      profile: [p],
      invoice: this.fb.group({
        number: ['', Validators.required],
        type: ['380', Validators.required],
        issue_date: [new Date().toISOString().split('T')[0], Validators.required],
        currency: ['EUR', Validators.required],
        purchase_order_ref: [''],
        note: [''],
      }),
      seller: this.fb.group({
        name: ['', Validators.required],
        vat_id: [''],
        siret: ['', [Validators.required, Validators.pattern(/^\d{14}$/)]],
        address: this.fb.group({
          street: ['', Validators.required],
          city: ['', Validators.required],
          postal_code: ['', Validators.required],
          country: ['FR', Validators.required],
        }),
        contact: this.fb.group({
          email: [''],
          phone: [''],
        }),
        bank: this.fb.group({
          iban: ['', requireBankInfo ? [Validators.required] : []],
          bic: [''],
        }),
      }),
      buyer: this.fb.group({
        name: ['', Validators.required],
        vat_id: [''],
        siret: ['', [Validators.required, Validators.pattern(/^\d{14}$/)]],
        address: this.fb.group({
          street: ['', Validators.required],
          city: ['', Validators.required],
          postal_code: ['', Validators.required],
          country: ['FR', Validators.required],
        }),
      }),
      lines: this.fb.array([this.createLine()]),
      payment: this.fb.group({
        terms: ['', requirePaymentTerms ? [Validators.required] : []],
        type_code: ['30'],
        due_date: [''],
      }),
    });
    
    this.cdr.detectChanges();
    console.log('Change detection forced after initForm');
  }

  get lines() {
    return this.invoiceForm?.get('lines') as FormArray;
  }

  addLine() {
    this.lines.push(this.createLine());
  }

  removeLine(i: number) {
    this.lines.removeAt(i);
  }

  private createLine(): FormGroup {
    return this.fb.group({
      description: ['', Validators.required],
      quantity: [1, [Validators.required, Validators.min(0.01)]],
      unit_price: [0, [Validators.required, Validators.min(0)]],
      unit: ['EA', Validators.required],
      vat_rate: [20, Validators.required],
    });
  }

  fillTestData() {
    this.invoiceForm.patchValue({
      invoice: {
        number: 'INV-' + Math.floor(Math.random() * 10000),
        type: '380',
        issue_date: new Date().toISOString().split('T')[0],
        currency: 'EUR',
        purchase_order_ref: 'PO-987654321',
        note: 'Merci pour votre achat. Ceci est une facture de test.',
      },
      seller: {
        name: 'Tech Corp France',
        vat_id: 'FR12345678901',
        siret: '12345678900012',
        address: {
          street: '123 Avenue de la République',
          city: 'Paris',
          postal_code: '75011',
          country: 'FR',
        },
        contact: {
          email: 'contact@techcorp.fr',
          phone: '01 02 03 04 05',
        },
        bank: this.invoiceForm.get('seller.bank')
          ? {
              iban: 'FR7630000000000000000000000',
              bic: 'TECHFRXX',
            }
          : undefined,
      },
      buyer: {
        name: 'Acme Solutions',
        vat_id: 'FR98765432109',
        siret: '98765432100098',
        address: {
          street: '45 Boulevard Haussmann',
          city: 'Paris',
          postal_code: '75009',
          country: 'FR',
        },
      },
      payment: this.invoiceForm.get('payment')
        ? {
            terms: 'Paiement à 30 jours fin de mois',
            type_code: '30',
            due_date: new Date(new Date().setMonth(new Date().getMonth() + 1))
              .toISOString()
              .split('T')[0],
          }
        : undefined,
    });

    // Reset lines and add test lines
    this.lines.clear();

    const line1 = this.createLine();
    line1.patchValue({
      description: 'Développement Backend Go',
      quantity: 10,
      unit: 'HUR',
      unit_price: 500,
      vat_rate: 20,
    });
    this.lines.push(line1);

    const line2 = this.createLine();
    line2.patchValue({
      description: 'Développement Frontend Angular',
      quantity: 8,
      unit: 'DAY',
      unit_price: 450,
      vat_rate: 20,
    });
    this.lines.push(line2);
  }

  onSubmit(): void {
    if (this.invoiceForm.invalid) {
      this.invoiceForm.markAllAsTouched();
      return;
    }

    this.isGenerating.set(true);
    this.error.set(null);

    const formValue = this.invoiceForm.value;

    // Compute totals automatically
    let subtotalExclVat = 0;
    let totalVat = 0;
    const vatBreakdownMap = new Map<number, { taxable: number; amount: number }>();

    for (const line of formValue.lines) {
      const qty = Number(line.quantity || 0);
      const price = Number(line.unit_price || 0);
      const rate = Number(line.vat_rate || 0);

      const lineTotalExcl = qty * price;
      subtotalExclVat += lineTotalExcl;

      const lineVat = lineTotalExcl * (rate / 100);
      totalVat += lineVat;

      if (!vatBreakdownMap.has(rate)) {
        vatBreakdownMap.set(rate, { taxable: 0, amount: 0 });
      }
      const b = vatBreakdownMap.get(rate)!;
      b.taxable += lineTotalExcl;
      b.amount += lineVat;
    }

    const totalInclVat = subtotalExclVat + totalVat;

    const invoicePayload: Invoice = {
      profile: formValue.profile,
      invoice: {
        ...formValue.invoice,
        issue_date: new Date(formValue.invoice.issue_date).toISOString(),
      },
      seller: {
        ...formValue.seller,
        global_id: formValue.seller.siret
          ? { scheme_id: '0009', value: formValue.seller.siret }
          : undefined,
      },
      buyer: {
        ...formValue.buyer,
        global_id: formValue.buyer.siret
          ? { scheme_id: '0009', value: formValue.buyer.siret }
          : undefined,
      },
      lines: formValue.lines.map(
        (l: any, i: number) => {
          const qty = Number(l.quantity || 0);
          const price = Number(l.unit_price || 0);
          const rate = Number(l.vat_rate || 0);
          
          const totalExcl = qty * price;
          const vatAmt = totalExcl * (rate / 100);
          return {
            id: (i + 1).toString(),
            description: l.description,
            quantity: qty,
            unit_price: price,
            vat_rate: rate,
            unit: l.unit || 'EA',
            vat_amount: vatAmt,
            total_excl_vat: totalExcl,
            total_incl_vat: totalExcl + vatAmt,
          };
        },
      ),
      totals: {
        subtotal_excl_vat: subtotalExclVat,
        total_vat: totalVat,
        total_incl_vat: totalInclVat,
        amount_due: totalInclVat,
        vat_breakdown: Array.from(vatBreakdownMap.entries()).map(([rate, v]) => ({
          rate: Number(rate),
          taxable_amount: v.taxable,
          vat_amount: v.amount,
        })),
      },
      payment: formValue.payment
        ? {
            ...formValue.payment,
            payment_means: formValue.payment.type_code ? { type_code: formValue.payment.type_code } : undefined,
            due_date: formValue.payment.due_date
              ? new Date(formValue.payment.due_date).toISOString()
              : undefined,
          }
        : undefined,
    };

    this.api
      .generateFacturX({
        invoice: invoicePayload,
        templateId: this.templateId(),
      })
      .subscribe({
        next: (pdfBlob) => {
          this.isGenerating.set(false);
          const url = window.URL.createObjectURL(pdfBlob);
          const a = document.createElement('a');
          a.href = url;
          a.download = `facture_${formValue.invoice.number}.pdf`;
          document.body.appendChild(a);
          a.click();
          window.URL.revokeObjectURL(url);
          document.body.removeChild(a);
        },
        error: (err) => {
          this.isGenerating.set(false);
          this.validationErrors.set([]);
          
          if (err?.error?.data?.errors && Array.isArray(err.error.data.errors)) {
            this.error.set('La facture contient des erreurs de validation :');
            this.validationErrors.set(err.error.data.errors);
          } else {
            const backendMsg = err?.error?.error || err?.message || 'Erreur inconnue';
            this.error.set('Erreur lors de la génération. Le serveur a répondu : ' + backendMsg);
          }
        },
      });
  }
}
