import { HttpClientTestingModule } from '@angular/common/http/testing';
import { ComponentFixture, TestBed } from '@angular/core/testing';
import { ActivatedRoute } from '@angular/router';
import { of } from 'rxjs';
import { Api } from '../../../core/services/api';
import { GenerateInvoice } from './generate-invoice';

describe('GenerateInvoice', () => {
  let component: GenerateInvoice;
  let fixture: ComponentFixture<GenerateInvoice>;

  beforeEach(async () => {
    await TestBed.configureTestingModule({
      imports: [GenerateInvoice, HttpClientTestingModule],
      providers: [
        {
          provide: ActivatedRoute,
          useValue: {
            snapshot: { paramMap: { get: () => 'test-id' } },
          },
        },
        {
          provide: Api,
          useValue: {
            getTemplate: () =>
              of({
                id: 'test-id',
                content: '// @profile: EN16931',
              }),
          },
        },
      ],
    }).compileComponents();

    fixture = TestBed.createComponent(GenerateInvoice);
    component = fixture.componentInstance;
    fixture.detectChanges();
  });

  it('should create the form for EN16931 profile', () => {
    expect((component as any).invoiceForm).toBeDefined();
    console.log('Form created successfully!');
  });
});
