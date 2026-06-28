export interface Template {
  id: string;
  name: string;
  description?: string;
  type?: string; // e.g. 'typst'
  content?: string; // Contenu Typst
  preview?: string; // URL de prévisualisation
  category?: TemplateCategory;
  tags?: string[];
  createdAt?: Date;
  updatedAt?: Date;
  isDefault?: boolean;
}

export enum TemplateCategory {
  INVOICE = 'INVOICE',
  QUOTE = 'QUOTE',
  CUSTOM = 'CUSTOM',
}
