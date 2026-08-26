export enum SymbolKind {
  Function = 'Function',
  Variable = 'Variable',
  Constant = 'Constant',
  Define = 'Define',
  Struct = 'Struct',
  Field = 'Field',
  Import = 'Import',
}

export interface StructField {
  name: string;
  type: string;
  docComment?: string;
}

export interface M3Symbol {
  name: string;
  kind: SymbolKind;
  detail: string;
  type?: string;
  storage?: string;
  bank?: string;
  signature?: string;
  value?: string;
  docComment?: string;
  line: number;
  column: number;
  fields?: StructField[];
}

export interface ParsedM3Document {
  symbols: Map<string, M3Symbol>;
  structs: Map<string, M3Symbol>;
  imports: string[];
}

export function parseM3Document(text: string): ParsedM3Document {
  const symbols = new Map<string, M3Symbol>();
  const structs = new Map<string, M3Symbol>();
  const imports: string[] = [];

  const lines = text.split(/\r?\n/);
  let pendingDocComment: string[] = [];
  let inBlockComment = false;
  let blockCommentBuffer: string[] = [];

  let inGroupBlock: 'var' | 'const' | 'define' | 'import' | null = null;
  let inStructDef: { symbol: M3Symbol } | null = null;

  for (let lineIndex = 0; lineIndex < lines.length; lineIndex++) {
    const rawLine = lines[lineIndex];
    let line = rawLine.trim();

    // Check multi-line block comments /* ... */
    if (inBlockComment) {
      const endIdx = line.indexOf('*/');
      if (endIdx !== -1) {
        blockCommentBuffer.push(line.substring(0, endIdx).trim());
        inBlockComment = false;
        pendingDocComment = [...blockCommentBuffer];
        blockCommentBuffer = [];
        line = line.substring(endIdx + 2).trim();
      } else {
        blockCommentBuffer.push(line.replace(/^\s*\*\s?/, '').trim());
        continue;
      }
    }

    if (line.startsWith('/*')) {
      const endIdx = line.indexOf('*/', 2);
      if (endIdx !== -1) {
        pendingDocComment.push(line.substring(2, endIdx).trim());
        line = line.substring(endIdx + 2).trim();
      } else {
        inBlockComment = true;
        blockCommentBuffer.push(line.substring(2).trim());
        continue;
      }
    }

    // Single-line comment //
    if (line.startsWith('//')) {
      const commentText = line.replace(/^\/\/\s?/, '').trim();
      pendingDocComment.push(commentText);
      continue;
    }

    // Strip inline comments for parsing code
    const inlineCommentIdx = line.indexOf('//');
    let codeLine = inlineCommentIdx !== -1 ? line.substring(0, inlineCommentIdx).trim() : line;

    if (codeLine.length === 0) {
      // Empty line clears pending comments if not inside a struct
      if (!inStructDef) {
        pendingDocComment = [];
      }
      continue;
    }

    const docStr = pendingDocComment.length > 0 ? pendingDocComment.join('\n') : undefined;

    // Check closing of groups or structs
    if (codeLine === ')' && inGroupBlock) {
      inGroupBlock = null;
      pendingDocComment = [];
      continue;
    }

    if (codeLine === '}' && inStructDef) {
      structs.set(inStructDef.symbol.name, inStructDef.symbol);
      symbols.set(inStructDef.symbol.name, inStructDef.symbol);
      inStructDef = null;
      pendingDocComment = [];
      continue;
    }

    // Inside struct definition
    if (inStructDef) {
      // Field: identifier type
      const fieldMatch = codeLine.match(/^([a-zA-Z_][a-zA-Z0-9_]*)\s+([*]?[a-zA-Z0-9_\[\]]+)/);
      if (fieldMatch) {
        const fieldName = fieldMatch[1];
        const fieldType = fieldMatch[2];
        inStructDef.symbol.fields = inStructDef.symbol.fields || [];
        inStructDef.symbol.fields.push({
          name: fieldName,
          type: fieldType,
          docComment: docStr,
        });
      }
      pendingDocComment = [];
      continue;
    }

    // Group block headers: var (, const (, define (, import (
    if (/^var\s*\($/.test(codeLine)) {
      inGroupBlock = 'var';
      pendingDocComment = [];
      continue;
    }
    if (/^const\s*\($/.test(codeLine)) {
      inGroupBlock = 'const';
      pendingDocComment = [];
      continue;
    }
    if (/^define\s*\($/.test(codeLine)) {
      inGroupBlock = 'define';
      pendingDocComment = [];
      continue;
    }
    if (/^import\s*\($/.test(codeLine)) {
      inGroupBlock = 'import';
      pendingDocComment = [];
      continue;
    }

    // Inside grouped var ( ... )
    if (inGroupBlock === 'var') {
      const varMatch = codeLine.match(/^([a-zA-Z_][a-zA-Z0-9_]*)\s+([*]?[a-zA-Z0-9_]+(?:\[\s*\d*\s*\])?)(?:\s+(zp|zeropage|ram|bss|wram|workram))?/);
      if (varMatch) {
        const varName = varMatch[1];
        const varType = varMatch[2];
        const varStorage = varMatch[3] || 'ram';
        const sym: M3Symbol = {
          name: varName,
          kind: SymbolKind.Variable,
          detail: `var ${varName} ${varType} ${varStorage}`,
          type: varType,
          storage: varStorage,
          docComment: docStr,
          line: lineIndex,
          column: rawLine.indexOf(varName),
        };
        symbols.set(varName, sym);
      }
      pendingDocComment = [];
      continue;
    }

    // Inside grouped const ( ... )
    if (inGroupBlock === 'const') {
      const constMatch = codeLine.match(/^([a-zA-Z_][a-zA-Z0-9_]*)(?:\s+([*]?[a-zA-Z0-9_]+(?:\[\s*\d*\s*\])?))?(?:\s+bank\s+(\d+|auto))?(?:\s*=\s*(.+))?/);
      if (constMatch) {
        const constName = constMatch[1];
        const constType = constMatch[2] || '';
        const constBank = constMatch[3];
        const constVal = constMatch[4];
        const sym: M3Symbol = {
          name: constName,
          kind: SymbolKind.Constant,
          detail: `const ${constName}${constType ? ' ' + constType : ''}${constBank ? ' bank ' + constBank : ''}${constVal ? ' = ' + constVal : ''}`,
          type: constType,
          bank: constBank,
          value: constVal,
          docComment: docStr,
          line: lineIndex,
          column: rawLine.indexOf(constName),
        };
        symbols.set(constName, sym);
      }
      pendingDocComment = [];
      continue;
    }

    // Inside grouped define ( ... )
    if (inGroupBlock === 'define') {
      const defMatch = codeLine.match(/^([a-zA-Z_][a-zA-Z0-9_]*)(?:\s*=?\s*(.+))?/);
      if (defMatch) {
        const defName = defMatch[1];
        const defVal = defMatch[2] || '';
        const sym: M3Symbol = {
          name: defName,
          kind: SymbolKind.Define,
          detail: `define ${defName} ${defVal}`,
          value: defVal,
          docComment: docStr,
          line: lineIndex,
          column: rawLine.indexOf(defName),
        };
        symbols.set(defName, sym);
      }
      pendingDocComment = [];
      continue;
    }

    // Inside grouped import ( ... )
    if (inGroupBlock === 'import') {
      const impMatch = codeLine.match(/^"([^"]+)"/);
      if (impMatch) {
        imports.push(impMatch[1]);
      }
      pendingDocComment = [];
      continue;
    }

    // Single-line import "path"
    const singleImportMatch = codeLine.match(/^import\s+"([^"]+)"/);
    if (singleImportMatch) {
      imports.push(singleImportMatch[1]);
      pendingDocComment = [];
      continue;
    }

    // Struct declaration: type <Name> struct {
    const structMatch = codeLine.match(/^type\s+([a-zA-Z_][a-zA-Z0-9_]*)\s+struct(?:\s*\{)?/);
    if (structMatch) {
      const structName = structMatch[1];
      const sym: M3Symbol = {
        name: structName,
        kind: SymbolKind.Struct,
        detail: `type ${structName} struct`,
        type: structName,
        docComment: docStr,
        line: lineIndex,
        column: rawLine.indexOf(structName),
        fields: [],
      };
      if (codeLine.includes('{')) {
        inStructDef = { symbol: sym };
      } else {
        structs.set(structName, sym);
        symbols.set(structName, sym);
      }
      pendingDocComment = [];
      continue;
    }

    // Function declaration: func <name>(<params>) [<returnType>] [bank <n>] {
    const funcMatch = codeLine.match(/^func\s+([a-zA-Z_][a-zA-Z0-9_]*)\s*\(([^)]*)\)(?:\s+(?!bank\b)([*a-zA-Z0-9_]+))?(?:\s+bank\s+([a-zA-Z0-9_]+))?/);
    if (funcMatch) {
      const funcName = funcMatch[1];
      const params = funcMatch[2].trim();
      const returnType = funcMatch[3] ? funcMatch[3].trim() : '';
      const bank = funcMatch[4] ? funcMatch[4].trim() : '';

      let signature = `func ${funcName}(${params})`;
      if (returnType) {
        signature += ` ${returnType}`;
      }
      if (bank) {
        signature += ` bank ${bank}`;
      }

      const sym: M3Symbol = {
        name: funcName,
        kind: SymbolKind.Function,
        detail: signature,
        type: returnType || 'void',
        bank: bank || undefined,
        signature: signature,
        docComment: docStr,
        line: lineIndex,
        column: rawLine.indexOf(funcName),
      };
      symbols.set(funcName, sym);
      pendingDocComment = [];
      continue;
    }

    // Single-line var declaration: var <name> <type>[length] [storage]
    const singleVarMatch = codeLine.match(/^var\s+([a-zA-Z_][a-zA-Z0-9_]*)\s+([*]?[a-zA-Z0-9_]+(?:\[\s*\d*\s*\])?)(?:\s+(zp|zeropage|ram|bss|wram|workram))?/);
    if (singleVarMatch) {
      const varName = singleVarMatch[1];
      const varType = singleVarMatch[2];
      const varStorage = singleVarMatch[3] || 'ram';
      const sym: M3Symbol = {
        name: varName,
        kind: SymbolKind.Variable,
        detail: `var ${varName} ${varType} ${varStorage}`,
        type: varType,
        storage: varStorage,
        docComment: docStr,
        line: lineIndex,
        column: rawLine.indexOf(varName),
      };
      symbols.set(varName, sym);
      pendingDocComment = [];
      continue;
    }

    // Single-line const declaration: const <name> <type>[length] [bank <n>] = <val>
    const singleConstMatch = codeLine.match(/^const\s+([a-zA-Z_][a-zA-Z0-9_]*)(?:\s+([*]?[a-zA-Z0-9_]+(?:\[\s*\d*\s*\])?))?(?:\s+bank\s+(\d+|auto))?(?:\s*=\s*(.+))?/);
    if (singleConstMatch) {
      const constName = singleConstMatch[1];
      const constType = singleConstMatch[2] || '';
      const constBank = singleConstMatch[3];
      const constVal = singleConstMatch[4];
      const sym: M3Symbol = {
        name: constName,
        kind: SymbolKind.Constant,
        detail: `const ${constName}${constType ? ' ' + constType : ''}${constBank ? ' bank ' + constBank : ''}${constVal ? ' = ' + constVal : ''}`,
        type: constType,
        bank: constBank,
        value: constVal,
        docComment: docStr,
        line: lineIndex,
        column: rawLine.indexOf(constName),
      };
      symbols.set(constName, sym);
      pendingDocComment = [];
      continue;
    }

    // Single-line define: define <name> <expr>
    const singleDefMatch = codeLine.match(/^define\s+([a-zA-Z_][a-zA-Z0-9_]*)(?:\s*=?\s*(.+))?/);
    if (singleDefMatch) {
      const defName = singleDefMatch[1];
      const defVal = singleDefMatch[2] || '';
      const sym: M3Symbol = {
        name: defName,
        kind: SymbolKind.Define,
        detail: `define ${defName} ${defVal}`,
        value: defVal,
        docComment: docStr,
        line: lineIndex,
        column: rawLine.indexOf(defName),
      };
      symbols.set(defName, sym);
      pendingDocComment = [];
      continue;
    }

    // Short variable declaration in code block: i := ... or var i uint8
    const shortVarMatch = codeLine.match(/([a-zA-Z_][a-zA-Z0-9_]*)\s*:=\s*/);
    if (shortVarMatch && !symbols.has(shortVarMatch[1])) {
      const varName = shortVarMatch[1];
      const sym: M3Symbol = {
        name: varName,
        kind: SymbolKind.Variable,
        detail: `var ${varName} (inferred)`,
        line: lineIndex,
        column: rawLine.indexOf(varName),
      };
      symbols.set(varName, sym);
    }

    pendingDocComment = [];
  }

  return { symbols, structs, imports };
}

/**
 * Given a variable type like "Actor" or "Actor[16]" or "*Actor",
 * resolves the underlying base struct type name.
 */
export function getBaseTypeName(typeStr?: string): string | undefined {
  if (!typeStr) return undefined;
  return typeStr.replace(/^\*/, '').replace(/\[\s*\d*\s*\]$/, '').trim();
}
