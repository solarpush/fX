export interface DocumentConfig {
  page: {
    paper: string;
    orientation: 'portrait' | 'landscape';
    margin: {
      top: string;
      bottom: string;
      left: string;
      right: string;
    };
  };
  text: {
    font: string;
    size: string;
    lang: string;
  };
}

export interface BlockProperty {
  type: 'text' | 'number' | 'color' | 'select' | 'boolean' | 'margin';
  label: string;
  value: any;
  options?: { label: string; value: string | number }[];
}

export interface BuilderBlock {
  id: string;
  type: string;
  title: string;
  properties: Record<string, BlockProperty>;
  typstCode: string;
  isCustom?: boolean;
}

export interface Template {
  id?: string;
  name: string;
  config: DocumentConfig;
  blocks: BuilderBlock[];
}
