import {
  ChangeDetectionStrategy,
  Component,
  computed,
  effect,
  input,
  output,
  signal,
} from '@angular/core';
import { CommonModule } from '@angular/common';
import { FormsModule } from '@angular/forms';
import {
  SchemaTreeNode,
  getSchemaArrayItemNodes,
} from '../../../../core/models/studio.models';
import { VariablePickerComponent } from '../variable-picker/variable-picker';

export type ConditionOp = 'exists' | 'empty' | '==' | '!=' | '>' | '<';

interface ParsedCondition {
  field: string;
  op: ConditionOp;
  rhs: string;
  rhsIsField: boolean;
  supported: boolean;
}

interface FieldOption {
  value: string;
  label: string;
  group: string;
}

const COMPARE_RX = /^(.+?)\s*(==|!=|>=|<=|>|<)\s*(.+)$/;
const NUM_RX = /^-?\d+(?:\.\d+)?$/;

@Component({
  selector: 'app-condition-builder',
  standalone: true,
  changeDetection: ChangeDetectionStrategy.OnPush,
  imports: [CommonModule, FormsModule, VariablePickerComponent],
  templateUrl: './condition-builder.html',
  styleUrl: './condition-builder.scss',
})
export class ConditionBuilderComponent {
  readonly value = input<string>('');
  readonly schemaNodes = input<readonly SchemaTreeNode[]>([]);
  readonly localRepeatPath = input<string | null>(null);

  readonly valueChange = output<string>();

  protected readonly mode = signal<'visual' | 'raw'>('visual');
  protected readonly field = signal('');
  protected readonly op = signal<ConditionOp>('exists');
  protected readonly rhs = signal('');
  protected readonly rhsIsField = signal(false);

  protected readonly operators: readonly { value: ConditionOp; label: string }[] = [
    { value: 'exists', label: 'Est rempli (existe)' },
    { value: 'empty', label: 'Est vide (!champ)' },
    { value: '==', label: 'Est égal à (==)' },
    { value: '!=', label: 'Est différent de (!=)' },
    { value: '>', label: 'Supérieur à (>)' },
    { value: '<', label: 'Inférieur à (<)' },
  ];

  protected readonly allFieldOptions = computed<FieldOption[]>(() => {
    const nodes = this.schemaNodes();
    const repeatPath = this.localRepeatPath()?.trim();
    const opts: FieldOption[] = [];

    const collect = (
      list: readonly SchemaTreeNode[],
      stripPrefix: string,
      prependRoot: boolean,
      groupName: string,
    ): void => {
      for (const n of list) {
        if (n.isArray) continue;
        if (n.type === 'object' && n.children.length > 0) {
          collect(n.children, stripPrefix, prependRoot, groupName);
          continue;
        }
        let rel = n.path;
        if (stripPrefix && rel.startsWith(stripPrefix)) {
          rel = rel.slice(stripPrefix.length);
        }
        rel = rel.replace(/^\./, '');
        const val = prependRoot ? `$root.${rel}` : rel;
        opts.push({ value: val, label: rel, group: groupName });
      }
    };

    if (repeatPath) {
      const itemNodes = getSchemaArrayItemNodes(nodes, repeatPath);
      collect(itemNodes, `${repeatPath}[].`, false, `Boucle locale (${repeatPath}[])`);
    }

    const prependRoot = !!repeatPath;
    for (const top of nodes) {
      if (top.isArray) continue;
      if (top.type === 'object' && top.children.length > 0) {
        collect(top.children, '', prependRoot, prependRoot ? `$root.${top.key}` : top.key);
      } else {
        const val = prependRoot ? `$root.${top.path}` : top.path;
        opts.push({
          value: val,
          label: top.path,
          group: prependRoot ? 'Racine ($root)' : 'Racine',
        });
      }
    }

    return opts;
  });

  protected readonly groupedFieldOptions = computed(() => {
    const map = new Map<string, FieldOption[]>();
    for (const opt of this.allFieldOptions()) {
      const arr = map.get(opt.group) ?? [];
      arr.push(opt);
      map.set(opt.group, arr);
    }
    return Array.from(map.entries()).map(([group, items]) => ({ group, items }));
  });

  protected readonly knownFieldSet = computed(
    () => new Set(this.allFieldOptions().map((o) => o.value)),
  );

  protected readonly needsRhs = computed(() => {
    const o = this.op();
    return o === '==' || o === '!=' || o === '>' || o === '<';
  });

  protected readonly hasCustomField = computed(() => {
    const f = this.field().trim();
    return !!f && !this.knownFieldSet().has(f);
  });

  private lastEmitted = '';

  constructor() {
    effect(() => {
      const raw = (this.value() ?? '').trim();
      if (raw === this.lastEmitted) {
        return;
      }
      this.lastEmitted = raw;
      const parsed = this.parseExpression(raw);
      this.field.set(parsed.field);
      this.op.set(parsed.op);
      this.rhs.set(parsed.rhs);
      this.rhsIsField.set(parsed.rhsIsField);
      if (!parsed.supported && raw) {
        this.mode.set('raw');
      }
    });
  }

  protected onFieldChange(nextField: string): void {
    this.field.set(nextField);
    this.emitVisual();
  }

  protected onOpChange(nextOp: ConditionOp): void {
    this.op.set(nextOp);
    this.emitVisual();
  }

  protected onRhsInput(nextRhs: string): void {
    this.rhs.set(nextRhs);
    const trimmed = nextRhs.trim();
    this.rhsIsField.set(
      this.knownFieldSet().has(trimmed) ||
        trimmed.startsWith('$root.') ||
        trimmed.startsWith('../'),
    );
    this.emitVisual();
  }

  protected onRhsFieldPicked(path: string): void {
    this.rhs.set(path);
    this.rhsIsField.set(true);
    this.emitVisual();
  }

  protected onRawChange(expr: string): void {
    const cleaned = expr.trim();
    this.lastEmitted = cleaned;
    const parsed = this.parseExpression(cleaned);
    this.field.set(parsed.field);
    this.op.set(parsed.op);
    this.rhs.set(parsed.rhs);
    this.rhsIsField.set(parsed.rhsIsField);
    this.valueChange.emit(cleaned);
  }

  protected clearCondition(): void {
    this.field.set('');
    this.op.set('exists');
    this.rhs.set('');
    this.rhsIsField.set(false);
    this.lastEmitted = '';
    this.valueChange.emit('');
  }

  private emitVisual(): void {
    const f = this.field().trim();
    if (!f) {
      if (this.lastEmitted !== '') {
        this.lastEmitted = '';
        this.valueChange.emit('');
      }
      return;
    }

    const o = this.op();
    let expr = '';

    if (o === 'exists') {
      expr = f;
    } else if (o === 'empty') {
      expr = `!${f}`;
    } else {
      const r = this.rhs().trim();
      if (!r) {
        // Wait until user enters comparison target so we don't emit invalid half-expression
        return;
      }
      const formattedRhs = this.formatRhs(o, r, this.rhsIsField());
      expr = `${f} ${o} ${formattedRhs}`;
    }

    this.lastEmitted = expr;
    this.valueChange.emit(expr);
  }

  private formatRhs(op: ConditionOp, raw: string, isField: boolean): string {
    if (
      (raw.startsWith('"') && raw.endsWith('"')) ||
      (raw.startsWith("'") && raw.endsWith("'"))
    ) {
      return raw;
    }
    if (isField || this.knownFieldSet().has(raw) || raw.startsWith('$root.') || raw.startsWith('../')) {
      return raw;
    }
    if (NUM_RX.test(raw) || raw === 'true' || raw === 'false' || raw === 'null') {
      return raw;
    }
    if (op === '>' || op === '<') {
      return raw;
    }
    const escaped = raw.replace(/\\/g, '\\\\').replace(/"/g, '\\"');
    return `"${escaped}"`;
  }

  private parseExpression(raw: string): ParsedCondition {
    if (!raw) {
      return { field: '', op: 'exists', rhs: '', rhsIsField: false, supported: true };
    }

    const m = raw.match(COMPARE_RX);
    if (m) {
      const lhs = m[1].trim();
      const opRaw = m[2];
      const rhsRaw = m[3].trim();

      if (opRaw === '>=' || opRaw === '<=') {
        return { field: lhs, op: opRaw === '>=' ? '>' : '<', rhs: rhsRaw, rhsIsField: false, supported: false };
      }

      const op = opRaw as ConditionOp;
      const isQuoted =
        rhsRaw.length >= 2 &&
        ((rhsRaw.startsWith('"') && rhsRaw.endsWith('"')) ||
          (rhsRaw.startsWith("'") && rhsRaw.endsWith("'")));
      const unquoted = isQuoted ? rhsRaw.slice(1, -1) : rhsRaw;
      const isField =
        !isQuoted &&
        !NUM_RX.test(rhsRaw) &&
        rhsRaw !== 'true' &&
        rhsRaw !== 'false' &&
        rhsRaw !== 'null';

      return { field: lhs, op, rhs: unquoted, rhsIsField: isField, supported: true };
    }

    if (raw.startsWith('!')) {
      return {
        field: raw.slice(1).trim(),
        op: 'empty',
        rhs: '',
        rhsIsField: false,
        supported: true,
      };
    }

    return { field: raw, op: 'exists', rhs: '', rhsIsField: false, supported: true };
  }
}
