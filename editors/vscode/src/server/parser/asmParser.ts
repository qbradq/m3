export enum AsmSymbolKind {
  Label = 'Label',
  LocalLabel = 'LocalLabel',
  Procedure = 'Procedure',
  Macro = 'Macro',
  Constant = 'Constant',
  Variable = 'Variable',
}

export interface AsmSymbol {
  name: string;
  kind: AsmSymbolKind;
  detail: string;
  value?: string;
  args?: string[];
  docComment?: string;
  line: number;
  column: number;
  bank?: string;
  scope?: string;
}

export interface ParsedAsmDocument {
  symbols: Map<string, AsmSymbol>;
  procs: Map<string, AsmSymbol>;
  macros: Map<string, AsmSymbol>;
}

export function parseAsmDocument(text: string): ParsedAsmDocument {
  const symbols = new Map<string, AsmSymbol>();
  const procs = new Map<string, AsmSymbol>();
  const macros = new Map<string, AsmSymbol>();

  const lines = text.split(/\r?\n/);
  let pendingDocComment: string[] = [];
  let currentBank: string | undefined = undefined;
  let currentScope: string | undefined = undefined;
  let currentMacro: { symbol: AsmSymbol } | null = null;
  let currentProc: { symbol: AsmSymbol } | null = null;

  for (let lineIndex = 0; lineIndex < lines.length; lineIndex++) {
    const rawLine = lines[lineIndex];
    let line = rawLine.trim();

    // Comment line (; ...)
    if (line.startsWith(';')) {
      const commentText = line.replace(/^;\s?/, '').trim();
      pendingDocComment.push(commentText);
      continue;
    }

    // Strip inline comments for code analysis
    const commentIdx = line.indexOf(';');
    let codeLine = commentIdx !== -1 ? line.substring(0, commentIdx).trim() : line;

    if (codeLine.length === 0) {
      if (!currentMacro && !currentProc) {
        pendingDocComment = [];
      }
      continue;
    }

    const docStr = pendingDocComment.length > 0 ? pendingDocComment.join('\n') : undefined;

    // Track .bank <index | auto>
    const bankMatch = codeLine.match(/^\.bank\s+([a-zA-Z0-9_]+)/i);
    if (bankMatch) {
      currentBank = bankMatch[1];
      pendingDocComment = [];
      continue;
    }

    // Track .endproc / .endmacro / .endscope
    if (/^\.endproc\b/i.test(codeLine)) {
      if (currentProc) {
        procs.set(currentProc.symbol.name, currentProc.symbol);
        symbols.set(currentProc.symbol.name, currentProc.symbol);
        currentProc = null;
      }
      pendingDocComment = [];
      continue;
    }

    if (/^\.endmacro\b/i.test(codeLine)) {
      if (currentMacro) {
        macros.set(currentMacro.symbol.name, currentMacro.symbol);
        symbols.set(currentMacro.symbol.name, currentMacro.symbol);
        currentMacro = null;
      }
      pendingDocComment = [];
      continue;
    }

    if (/^\.endscope\b/i.test(codeLine)) {
      currentScope = undefined;
      pendingDocComment = [];
      continue;
    }

    // .proc <name>
    const procMatch = codeLine.match(/^\.proc\s+([a-zA-Z_][a-zA-Z0-9_]*)/i);
    if (procMatch) {
      const procName = procMatch[1];
      const sym: AsmSymbol = {
        name: procName,
        kind: AsmSymbolKind.Procedure,
        detail: `.proc ${procName}${currentBank !== undefined ? ' (bank ' + currentBank + ')' : ''}`,
        docComment: docStr,
        line: lineIndex,
        column: rawLine.indexOf(procName),
        bank: currentBank,
      };
      currentProc = { symbol: sym };
      symbols.set(procName, sym);
      pendingDocComment = [];
      continue;
    }

    // .macro <name> [<args>...]
    const macroMatch = codeLine.match(/^\.macro\s+([a-zA-Z_][a-zA-Z0-9_]*)(?:\s+(.+))?/i);
    if (macroMatch) {
      const macroName = macroMatch[1];
      const rawArgs = macroMatch[2] ? macroMatch[2].split(',').map((a) => a.trim()) : [];
      const sym: AsmSymbol = {
        name: macroName,
        kind: AsmSymbolKind.Macro,
        detail: `.macro ${macroName} ${rawArgs.join(', ')}`,
        args: rawArgs,
        docComment: docStr,
        line: lineIndex,
        column: rawLine.indexOf(macroName),
      };
      currentMacro = { symbol: sym };
      symbols.set(macroName, sym);
      pendingDocComment = [];
      continue;
    }

    // .scope <name>
    const scopeMatch = codeLine.match(/^\.scope\s+([a-zA-Z_][a-zA-Z0-9_]*)/i);
    if (scopeMatch) {
      currentScope = scopeMatch[1];
      pendingDocComment = [];
      continue;
    }

    // .define / .def <NAME> <expr>
    const defMatch = codeLine.match(/^\.(?:define|def)\s+([a-zA-Z_][a-zA-Z0-9_]*)\s+(.+)/i);
    if (defMatch) {
      const defName = defMatch[1];
      const defVal = defMatch[2].trim();
      const fullName = currentScope ? `${currentScope}::${defName}` : defName;
      const sym: AsmSymbol = {
        name: fullName,
        kind: AsmSymbolKind.Constant,
        detail: `.define ${fullName} ${defVal}`,
        value: defVal,
        docComment: docStr,
        line: lineIndex,
        column: rawLine.indexOf(defName),
        scope: currentScope,
      };
      symbols.set(fullName, sym);
      if (currentScope) symbols.set(defName, sym);
      pendingDocComment = [];
      continue;
    }

    // NAME = <expr> or NAME .set <expr> or NAME .equ <expr>
    const assignMatch = codeLine.match(/^([a-zA-Z_][a-zA-Z0-9_]*)\s*(?:=|\.set|\.equ)\s*(.+)/i);
    if (assignMatch) {
      const assignName = assignMatch[1];
      const assignVal = assignMatch[2].trim();
      const fullName = currentScope ? `${currentScope}::${assignName}` : assignName;
      const sym: AsmSymbol = {
        name: fullName,
        kind: AsmSymbolKind.Constant,
        detail: `${fullName} = ${assignVal}`,
        value: assignVal,
        docComment: docStr,
        line: lineIndex,
        column: rawLine.indexOf(assignName),
        scope: currentScope,
      };
      symbols.set(fullName, sym);
      if (currentScope) symbols.set(assignName, sym);
      pendingDocComment = [];
      continue;
    }

    // Global Label: label: or label: .res 1
    const labelMatch = codeLine.match(/^([a-zA-Z_][a-zA-Z0-9_]*):\s*(.*)/);
    if (labelMatch) {
      const labelName = labelMatch[1];
      const remainder = labelMatch[2].trim();
      const fullName = currentScope ? `${currentScope}::${labelName}` : labelName;

      let isVar = false;
      let detail = `${fullName}:${currentBank !== undefined ? ' (bank ' + currentBank + ')' : ''}`;

      // Check RAM allocation directive on same line
      const resMatch = remainder.match(/^\.(?:res|reserve|zp|zeropage|ram|bss|wram|prgram|sram)\s*(.*)/i);
      if (resMatch) {
        isVar = true;
        detail = `${fullName}: ${remainder}`;
      }

      const sym: AsmSymbol = {
        name: fullName,
        kind: isVar ? AsmSymbolKind.Variable : AsmSymbolKind.Label,
        detail: detail,
        docComment: docStr,
        line: lineIndex,
        column: rawLine.indexOf(labelName),
        bank: currentBank,
        scope: currentScope,
      };
      symbols.set(fullName, sym);
      if (currentScope) symbols.set(labelName, sym);
      pendingDocComment = [];
      continue;
    }

    // Local Label: @local:
    const localMatch = codeLine.match(/^(@[a-zA-Z0-9_]+):/);
    if (localMatch) {
      const localName = localMatch[1];
      const sym: AsmSymbol = {
        name: localName,
        kind: AsmSymbolKind.LocalLabel,
        detail: `${localName} (local label)`,
        docComment: docStr,
        line: lineIndex,
        column: rawLine.indexOf(localName),
        bank: currentBank,
      };
      symbols.set(localName, sym);
      pendingDocComment = [];
      continue;
    }

    pendingDocComment = [];
  }

  return { symbols, procs, macros };
}
