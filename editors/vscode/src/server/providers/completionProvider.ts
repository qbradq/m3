import {
  CompletionItem,
  CompletionItemKind,
  MarkupKind,
  Position,
} from 'vscode-languageserver/node';
import { TextDocument } from 'vscode-languageserver-textdocument';
import { M3_TYPES, M3_STORAGE, M3_INTRINSICS, M3_KEYWORDS, DocEntry } from '../data/m3Docs';
import { ASM_INSTRUCTIONS, ASM_DIRECTIVES, NES_REGISTERS } from '../data/asmDocs';
import { ParsedM3Document, SymbolKind, getBaseTypeName } from '../parser/m3Parser';
import { ParsedAsmDocument, AsmSymbolKind } from '../parser/asmParser';

function docEntryToCompletionItem(entry: DocEntry): CompletionItem {
  return {
    label: entry.name,
    kind: entry.kind,
    detail: entry.detail,
    documentation: {
      kind: MarkupKind.Markdown,
      value: entry.documentation,
    },
    insertText: entry.snippet || entry.name,
    insertTextFormat: entry.insertTextFormat,
  };
}

export function getM3Completions(
  document: TextDocument,
  position: Position,
  parsedDoc: ParsedM3Document
): CompletionItem[] {
  const lineText = document.getText({
    start: { line: position.line, character: 0 },
    end: position,
  });

  // Check for struct member access: e.g. "actor." or "actors[i]."
  const memberMatch = lineText.match(/([a-zA-Z_][a-zA-Z0-9_]*(?:\[[^\]]*\])?)\.([a-zA-Z0-9_]*)$/);
  if (memberMatch) {
    const rawTarget = memberMatch[1];
    const varName = rawTarget.replace(/\[.*\]/, '').trim();
    const varSym = parsedDoc.symbols.get(varName);

    if (varSym && varSym.type) {
      const baseType = getBaseTypeName(varSym.type);
      const structSym = baseType ? parsedDoc.structs.get(baseType) : undefined;
      if (structSym && structSym.fields) {
        return structSym.fields.map((f) => ({
          label: f.name,
          kind: CompletionItemKind.Field,
          detail: `${structSym.name}.${f.name}: ${f.type}`,
          documentation: f.docComment
            ? {
                kind: MarkupKind.Markdown,
                value: f.docComment,
              }
            : undefined,
        }));
      }
    }
  }

  const items: CompletionItem[] = [];

  // 1. User-defined symbols in document
  for (const sym of parsedDoc.symbols.values()) {
    let itemKind: CompletionItemKind = CompletionItemKind.Variable;
    if (sym.kind === SymbolKind.Function) {
      itemKind = CompletionItemKind.Function;
    } else if (sym.kind === SymbolKind.Constant || sym.kind === SymbolKind.Define) {
      itemKind = CompletionItemKind.Constant;
    } else if (sym.kind === SymbolKind.Struct) {
      itemKind = CompletionItemKind.Struct;
    }

    items.push({
      label: sym.name,
      kind: itemKind,
      detail: sym.detail,
      documentation: sym.docComment
        ? {
            kind: MarkupKind.Markdown,
            value: sym.docComment,
          }
        : undefined,
    });
  }

  // 2. Built-in types
  for (const entry of Object.values(M3_TYPES)) {
    items.push(docEntryToCompletionItem(entry));
  }

  // 3. Storage specifiers
  for (const entry of Object.values(M3_STORAGE)) {
    items.push(docEntryToCompletionItem(entry));
  }

  // 4. Built-in intrinsics
  for (const entry of Object.values(M3_INTRINSICS)) {
    items.push(docEntryToCompletionItem(entry));
  }

  // 5. Language keywords and snippets
  for (const entry of Object.values(M3_KEYWORDS)) {
    items.push(docEntryToCompletionItem(entry));
  }

  return items;
}

export function getAsmCompletions(
  document: TextDocument,
  position: Position,
  parsedDoc: ParsedAsmDocument
): CompletionItem[] {
  const lineText = document.getText({
    start: { line: position.line, character: 0 },
    end: position,
  });

  const items: CompletionItem[] = [];

  // 1. If typing a directive (starts with dot '.')
  const dotMatch = lineText.match(/\.([a-zA-Z0-9_]*)$/);
  if (dotMatch) {
    for (const entry of Object.values(ASM_DIRECTIVES)) {
      items.push(docEntryToCompletionItem(entry));
    }
    return items;
  }

  // 2. User-defined symbols (labels, constants, procs, macros)
  for (const sym of parsedDoc.symbols.values()) {
    let itemKind: CompletionItemKind = CompletionItemKind.Variable;
    if (sym.kind === AsmSymbolKind.Procedure) {
      itemKind = CompletionItemKind.Function;
    } else if (sym.kind === AsmSymbolKind.Macro) {
      itemKind = CompletionItemKind.Snippet;
    } else if (sym.kind === AsmSymbolKind.Constant) {
      itemKind = CompletionItemKind.Constant;
    } else if (sym.kind === AsmSymbolKind.Label || sym.kind === AsmSymbolKind.LocalLabel) {
      itemKind = CompletionItemKind.Reference;
    }

    items.push({
      label: sym.name,
      kind: itemKind,
      detail: sym.detail,
      documentation: sym.docComment
        ? {
            kind: MarkupKind.Markdown,
            value: sym.docComment,
          }
        : undefined,
    });
  }

  // 3. 6502 Instruction Mnemonics
  for (const entry of Object.values(ASM_INSTRUCTIONS)) {
    items.push({
      label: entry.name,
      kind: entry.kind,
      detail: entry.detail,
      documentation: {
        kind: MarkupKind.Markdown,
        value: entry.documentation,
      },
    });
  }

  // 4. Assembler Directives (with leading dot)
  for (const entry of Object.values(ASM_DIRECTIVES)) {
    items.push(docEntryToCompletionItem(entry));
  }

  // 5. NES Hardware Registers
  for (const entry of Object.values(NES_REGISTERS)) {
    items.push(docEntryToCompletionItem(entry));
  }

  return items;
}
