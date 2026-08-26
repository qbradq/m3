import * as fs from 'fs';
import * as path from 'path';
import { fileURLToPath } from 'url';
import { M3_STDLIB } from '../data/m3StdLib';

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
  packageName?: string;
}

export interface ParsedM3Document {
  packageName?: string;
  symbols: Map<string, M3Symbol>;
  structs: Map<string, M3Symbol>;
  imports: string[];
  importedPackages: Map<string, ParsedM3Document>;
}

export function parseM3Document(text: string, docUri?: string): ParsedM3Document {
  let packageName: string | undefined = undefined;
  const symbols = new Map<string, M3Symbol>();
  const structs = new Map<string, M3Symbol>();
  const imports: string[] = [];
  const importedPackages = new Map<string, ParsedM3Document>();

  const lines = text.split(/\r?\n/);
  let pendingDocComment: string[] = [];
  let inBlockComment = false;
  let blockCommentBuffer: string[] = [];

  let inGroupBlock: 'var' | 'const' | 'define' | 'import' | null = null;
  let inStructDef: { symbol: M3Symbol } | null = null;
  let inAsmBlock = false;

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

    // Single-line comment // or assembly comment ; (if in asm)
    if (line.startsWith('//') || (inAsmBlock && line.startsWith(';'))) {
      const commentText = line.replace(/^(?:\/\/|;)\s?/, '').trim();
      pendingDocComment.push(commentText);
      continue;
    }

    // Strip inline comments for parsing code
    const inlineCommentIdx = line.indexOf('//');
    let codeLine = inlineCommentIdx !== -1 ? line.substring(0, inlineCommentIdx).trim() : line;
    if (inAsmBlock) {
      const asmCommentIdx = codeLine.indexOf(';');
      if (asmCommentIdx !== -1) {
        codeLine = codeLine.substring(0, asmCommentIdx).trim();
      }
    }

    if (codeLine.length === 0) {
      if (!inStructDef) {
        pendingDocComment = [];
      }
      continue;
    }

    const docStr = pendingDocComment.length > 0 ? pendingDocComment.join('\n') : undefined;

    // Track package declaration: package <name>
    const pkgMatch = codeLine.match(/^package\s+([a-zA-Z_][a-zA-Z0-9_]*)/);
    if (pkgMatch) {
      packageName = pkgMatch[1];
      pendingDocComment = [];
      continue;
    }

    // Check closing of groups, structs, or asm blocks
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

    if (codeLine === '}' && inAsmBlock) {
      inAsmBlock = false;
      pendingDocComment = [];
      continue;
    }

    if (/^asm\s*\{/.test(codeLine)) {
      inAsmBlock = true;
      pendingDocComment = [];
      continue;
    }

    // Inside asm block in m3 (e.g. data export labels)
    if (inAsmBlock) {
      const labelMatch = codeLine.match(/^([a-zA-Z_][a-zA-Z0-9_]*):/);
      if (labelMatch) {
        const labelName = labelMatch[1];
        const sym: M3Symbol = {
          name: labelName,
          kind: SymbolKind.Constant,
          detail: `const ${labelName} (asm label)`,
          docComment: docStr,
          line: lineIndex,
          column: rawLine.indexOf(labelName),
          packageName: packageName,
        };
        symbols.set(labelName, sym);
      }
      pendingDocComment = [];
      continue;
    }

    // Inside struct definition
    if (inStructDef) {
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
          packageName: packageName,
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
          packageName: packageName,
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
          packageName: packageName,
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
        packageName: packageName,
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
        packageName: packageName,
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
        packageName: packageName,
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
        packageName: packageName,
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
        packageName: packageName,
      };
      symbols.set(defName, sym);
      pendingDocComment = [];
      continue;
    }

    // Short variable declaration: i := ...
    const shortVarMatch = codeLine.match(/([a-zA-Z_][a-zA-Z0-9_]*)\s*:=\s*/);
    if (shortVarMatch && !symbols.has(shortVarMatch[1])) {
      const varName = shortVarMatch[1];
      const sym: M3Symbol = {
        name: varName,
        kind: SymbolKind.Variable,
        detail: `var ${varName} (inferred)`,
        line: lineIndex,
        column: rawLine.indexOf(varName),
        packageName: packageName,
      };
      symbols.set(varName, sym);
    }

    pendingDocComment = [];
  }

  const parsedDoc: ParsedM3Document = { packageName, symbols, structs, imports, importedPackages };

  // Resolve imports
  resolveImportsForDocument(parsedDoc, docUri);

  return parsedDoc;
}

function resolveImportsForDocument(parsedDoc: ParsedM3Document, docUri?: string) {
  let docDir: string | undefined = undefined;
  if (docUri) {
    try {
      if (docUri.startsWith('file://')) {
        docDir = path.dirname(fileURLToPath(docUri));
      } else {
        docDir = path.dirname(docUri);
      }
    } catch {
      docDir = undefined;
    }
  }

  for (const importPath of parsedDoc.imports) {
    const baseName = path.basename(importPath, '.m3');
    let content: string | undefined = undefined;
    let resolvedUri: string | undefined = undefined;

    // 1. Check embedded standard library
    const libKey = path.basename(importPath);
    if (M3_STDLIB[libKey]) {
      content = M3_STDLIB[libKey];
    } else if (docDir) {
      // 2. Relative import (e.g. "./data/data.m3" or "../common/utils.m3")
      if (importPath.startsWith('./') || importPath.startsWith('../')) {
        const targetPath = path.resolve(docDir, importPath);
        if (fs.existsSync(targetPath)) {
          try {
            content = fs.readFileSync(targetPath, 'utf8');
            resolvedUri = targetPath;
          } catch {
            content = undefined;
          }
        }
      } else {
        // 3. Search standard lib in workspace hierarchy
        let currentDir = docDir;
        for (let i = 0; i < 6; i++) {
          const checkPath = path.join(currentDir, 'pkg', 'data', 'lib', importPath);
          if (fs.existsSync(checkPath)) {
            try {
              content = fs.readFileSync(checkPath, 'utf8');
              resolvedUri = checkPath;
              break;
            } catch {
              // ignore
            }
          }
          const parent = path.dirname(currentDir);
          if (parent === currentDir) break;
          currentDir = parent;
        }
      }
    }

    if (content) {
      // Parse imported document without recursive infinite loop
      const importedDoc = parseM3DocumentWithoutImports(content);
      const pkgName = importedDoc.packageName || baseName;
      parsedDoc.importedPackages.set(pkgName, importedDoc);
    }
  }
}

function parseM3DocumentWithoutImports(text: string): ParsedM3Document {
  // Parses symbols, structs, and package name from source without triggering resolveImports
  let packageName: string | undefined = undefined;
  const symbols = new Map<string, M3Symbol>();
  const structs = new Map<string, M3Symbol>();
  const imports: string[] = [];
  const importedPackages = new Map<string, ParsedM3Document>();

  const lines = text.split(/\r?\n/);
  let pendingDocComment: string[] = [];
  let inBlockComment = false;
  let blockCommentBuffer: string[] = [];
  let inGroupBlock: 'var' | 'const' | 'define' | 'import' | null = null;
  let inStructDef: { symbol: M3Symbol } | null = null;
  let inAsmBlock = false;

  for (let lineIndex = 0; lineIndex < lines.length; lineIndex++) {
    const rawLine = lines[lineIndex];
    let line = rawLine.trim();

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

    if (line.startsWith('//') || (inAsmBlock && line.startsWith(';'))) {
      const commentText = line.replace(/^(?:\/\/|;)\s?/, '').trim();
      pendingDocComment.push(commentText);
      continue;
    }

    const inlineCommentIdx = line.indexOf('//');
    let codeLine = inlineCommentIdx !== -1 ? line.substring(0, inlineCommentIdx).trim() : line;
    if (inAsmBlock) {
      const asmCommentIdx = codeLine.indexOf(';');
      if (asmCommentIdx !== -1) {
        codeLine = codeLine.substring(0, asmCommentIdx).trim();
      }
    }

    if (codeLine.length === 0) {
      if (!inStructDef) pendingDocComment = [];
      continue;
    }

    const docStr = pendingDocComment.length > 0 ? pendingDocComment.join('\n') : undefined;

    const pkgMatch = codeLine.match(/^package\s+([a-zA-Z_][a-zA-Z0-9_]*)/);
    if (pkgMatch) {
      packageName = pkgMatch[1];
      pendingDocComment = [];
      continue;
    }

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

    if (codeLine === '}' && inAsmBlock) {
      inAsmBlock = false;
      pendingDocComment = [];
      continue;
    }

    if (/^asm\s*\{/.test(codeLine)) {
      inAsmBlock = true;
      pendingDocComment = [];
      continue;
    }

    if (inAsmBlock) {
      const labelMatch = codeLine.match(/^([a-zA-Z_][a-zA-Z0-9_]*):/);
      if (labelMatch) {
        const labelName = labelMatch[1];
        const sym: M3Symbol = {
          name: labelName,
          kind: SymbolKind.Constant,
          detail: `const ${labelName} (asm label)`,
          docComment: docStr,
          line: lineIndex,
          column: rawLine.indexOf(labelName),
          packageName: packageName,
        };
        symbols.set(labelName, sym);
      }
      pendingDocComment = [];
      continue;
    }

    if (inStructDef) {
      const fieldMatch = codeLine.match(/^([a-zA-Z_][a-zA-Z0-9_]*)\s+([*]?[a-zA-Z0-9_\[\]]+)/);
      if (fieldMatch) {
        inStructDef.symbol.fields = inStructDef.symbol.fields || [];
        inStructDef.symbol.fields.push({
          name: fieldMatch[1],
          type: fieldMatch[2],
          docComment: docStr,
        });
      }
      pendingDocComment = [];
      continue;
    }

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

    if (inGroupBlock === 'var') {
      const varMatch = codeLine.match(/^([a-zA-Z_][a-zA-Z0-9_]*)\s+([*]?[a-zA-Z0-9_]+(?:\[\s*\d*\s*\])?)(?:\s+(zp|zeropage|ram|bss|wram|workram))?/);
      if (varMatch) {
        const varName = varMatch[1];
        const varType = varMatch[2];
        const varStorage = varMatch[3] || 'ram';
        symbols.set(varName, {
          name: varName,
          kind: SymbolKind.Variable,
          detail: `var ${varName} ${varType} ${varStorage}`,
          type: varType,
          storage: varStorage,
          docComment: docStr,
          line: lineIndex,
          column: rawLine.indexOf(varName),
          packageName: packageName,
        });
      }
      pendingDocComment = [];
      continue;
    }

    if (inGroupBlock === 'const') {
      const constMatch = codeLine.match(/^([a-zA-Z_][a-zA-Z0-9_]*)(?:\s+([*]?[a-zA-Z0-9_]+(?:\[\s*\d*\s*\])?))?(?:\s+bank\s+(\d+|auto))?(?:\s*=\s*(.+))?/);
      if (constMatch) {
        const constName = constMatch[1];
        const constType = constMatch[2] || '';
        const constBank = constMatch[3];
        const constVal = constMatch[4];
        symbols.set(constName, {
          name: constName,
          kind: SymbolKind.Constant,
          detail: `const ${constName}${constType ? ' ' + constType : ''}${constBank ? ' bank ' + constBank : ''}${constVal ? ' = ' + constVal : ''}`,
          type: constType,
          bank: constBank,
          value: constVal,
          docComment: docStr,
          line: lineIndex,
          column: rawLine.indexOf(constName),
          packageName: packageName,
        });
      }
      pendingDocComment = [];
      continue;
    }

    if (inGroupBlock === 'define') {
      const defMatch = codeLine.match(/^([a-zA-Z_][a-zA-Z0-9_]*)(?:\s*=?\s*(.+))?/);
      if (defMatch) {
        const defName = defMatch[1];
        const defVal = defMatch[2] || '';
        symbols.set(defName, {
          name: defName,
          kind: SymbolKind.Define,
          detail: `define ${defName} ${defVal}`,
          value: defVal,
          docComment: docStr,
          line: lineIndex,
          column: rawLine.indexOf(defName),
          packageName: packageName,
        });
      }
      pendingDocComment = [];
      continue;
    }

    if (inGroupBlock === 'import') {
      const impMatch = codeLine.match(/^"([^"]+)"/);
      if (impMatch) imports.push(impMatch[1]);
      pendingDocComment = [];
      continue;
    }

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
        packageName: packageName,
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

    const funcMatch = codeLine.match(/^func\s+([a-zA-Z_][a-zA-Z0-9_]*)\s*\(([^)]*)\)(?:\s+(?!bank\b)([*a-zA-Z0-9_]+))?(?:\s+bank\s+([a-zA-Z0-9_]+))?/);
    if (funcMatch) {
      const funcName = funcMatch[1];
      const params = funcMatch[2].trim();
      const returnType = funcMatch[3] ? funcMatch[3].trim() : '';
      const bank = funcMatch[4] ? funcMatch[4].trim() : '';

      let signature = `func ${funcName}(${params})`;
      if (returnType) signature += ` ${returnType}`;
      if (bank) signature += ` bank ${bank}`;

      symbols.set(funcName, {
        name: funcName,
        kind: SymbolKind.Function,
        detail: signature,
        type: returnType || 'void',
        bank: bank || undefined,
        signature: signature,
        docComment: docStr,
        line: lineIndex,
        column: rawLine.indexOf(funcName),
        packageName: packageName,
      });
      pendingDocComment = [];
      continue;
    }

    const singleVarMatch = codeLine.match(/^var\s+([a-zA-Z_][a-zA-Z0-9_]*)\s+([*]?[a-zA-Z0-9_]+(?:\[\s*\d*\s*\])?)(?:\s+(zp|zeropage|ram|bss|wram|workram))?/);
    if (singleVarMatch) {
      const varName = singleVarMatch[1];
      const varType = singleVarMatch[2];
      const varStorage = singleVarMatch[3] || 'ram';
      symbols.set(varName, {
        name: varName,
        kind: SymbolKind.Variable,
        detail: `var ${varName} ${varType} ${varStorage}`,
        type: varType,
        storage: varStorage,
        docComment: docStr,
        line: lineIndex,
        column: rawLine.indexOf(varName),
        packageName: packageName,
      });
      pendingDocComment = [];
      continue;
    }

    const singleConstMatch = codeLine.match(/^const\s+([a-zA-Z_][a-zA-Z0-9_]*)(?:\s+([*]?[a-zA-Z0-9_]+(?:\[\s*\d*\s*\])?))?(?:\s+bank\s+(\d+|auto))?(?:\s*=\s*(.+))?/);
    if (singleConstMatch) {
      const constName = singleConstMatch[1];
      const constType = singleConstMatch[2] || '';
      const constBank = singleConstMatch[3];
      const constVal = singleConstMatch[4];
      symbols.set(constName, {
        name: constName,
        kind: SymbolKind.Constant,
        detail: `const ${constName}${constType ? ' ' + constType : ''}${constBank ? ' bank ' + constBank : ''}${constVal ? ' = ' + constVal : ''}`,
        type: constType,
        bank: constBank,
        value: constVal,
        docComment: docStr,
        line: lineIndex,
        column: rawLine.indexOf(constName),
        packageName: packageName,
      });
      pendingDocComment = [];
      continue;
    }

    const singleDefMatch = codeLine.match(/^define\s+([a-zA-Z_][a-zA-Z0-9_]*)(?:\s*=?\s*(.+))?/);
    if (singleDefMatch) {
      const defName = singleDefMatch[1];
      const defVal = singleDefMatch[2] || '';
      symbols.set(defName, {
        name: defName,
        kind: SymbolKind.Define,
        detail: `define ${defName} ${defVal}`,
        value: defVal,
        docComment: docStr,
        line: lineIndex,
        column: rawLine.indexOf(defName),
        packageName: packageName,
      });
      pendingDocComment = [];
      continue;
    }

    pendingDocComment = [];
  }

  return { packageName, symbols, structs, imports, importedPackages };
}

/**
 * Given a variable type like "Actor" or "Actor[16]" or "*Actor",
 * resolves the underlying base struct type name.
 */
export function getBaseTypeName(typeStr?: string): string | undefined {
  if (!typeStr) return undefined;
  return typeStr.replace(/^\*/, '').replace(/\[\s*\d*\s*\]$/, '').trim();
}
