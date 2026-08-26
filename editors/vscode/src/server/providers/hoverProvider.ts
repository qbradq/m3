import { Hover, MarkupKind, Position } from 'vscode-languageserver/node';
import { TextDocument } from 'vscode-languageserver-textdocument';
import { M3_TYPES, M3_STORAGE, M3_INTRINSICS, M3_KEYWORDS } from '../data/m3Docs';
import { ASM_INSTRUCTIONS, ASM_DIRECTIVES, NES_REGISTERS } from '../data/asmDocs';
import { ParsedM3Document, SymbolKind } from '../parser/m3Parser';
import { ParsedAsmDocument, AsmSymbolKind } from '../parser/asmParser';

export interface WordContext {
  rawWord: string;
  subWord: string;
  prefix?: string;
}

/**
 * Extracts the word and context under cursor, splitting dotted member access if applicable.
 */
export function getWordAtPosition(document: TextDocument, position: Position): WordContext | null {
  const text = document.getText({
    start: { line: position.line, character: 0 },
    end: { line: position.line + 1, character: 0 },
  });

  const line = text.replace(/\r?\n$/, '');
  const charIdx = position.character;

  if (charIdx < 0 || charIdx > line.length) {
    return null;
  }

  const regex = /([.@a-zA-Z_][a-zA-Z0-9_.]*)/g;
  let match: RegExpExecArray | null;

  while ((match = regex.exec(line)) !== null) {
    const start = match.index;
    const end = start + match[0].length;
    if (charIdx >= start && charIdx <= end) {
      const full = match[0];
      if (full.includes('.') && !full.startsWith('.')) {
        const dotIdx = full.indexOf('.');
        const splitPoint = start + dotIdx;
        if (charIdx <= splitPoint) {
          // Cursor is on package prefix before dot
          return {
            rawWord: full,
            subWord: full.substring(0, dotIdx),
          };
        } else {
          // Cursor is on member after dot
          return {
            rawWord: full,
            subWord: full.substring(dotIdx + 1),
            prefix: full.substring(0, dotIdx),
          };
        }
      }
      return {
        rawWord: full,
        subWord: full,
      };
    }
  }

  return null;
}

export function getM3Hover(
  document: TextDocument,
  position: Position,
  parsedDoc: ParsedM3Document
): Hover | null {
  const wordCtx = getWordAtPosition(document, position);
  if (!wordCtx) return null;

  const { rawWord, subWord, prefix } = wordCtx;

  // 1. Check if hovering on member of imported package: e.g. cursor on "Disable" in "ppu.Disable"
  if (prefix && parsedDoc.importedPackages.has(prefix)) {
    const pkg = parsedDoc.importedPackages.get(prefix)!;
    const sym = pkg.symbols.get(subWord);
    if (sym) {
      let md = `\`\`\`go\nfunc ${prefix}.${sym.detail.replace(/^func\s+/, '')}\n\`\`\``;
      if (sym.kind === SymbolKind.Constant || sym.kind === SymbolKind.Define) {
        md = `\`\`\`go\n${prefix}.${sym.detail}\n\`\`\``;
      }
      if (sym.docComment) {
        md += `\n\n${sym.docComment}`;
      }
      return {
        contents: {
          kind: MarkupKind.Markdown,
          value: md,
        },
      };
    }
  }

  // 2. Check if hovering on imported package name: e.g. cursor on "ppu" in "ppu.Disable" or standalone "ppu"
  if (parsedDoc.importedPackages.has(subWord)) {
    const pkg = parsedDoc.importedPackages.get(subWord)!;
    let md = `\`\`\`go\npackage ${subWord}\n\`\`\`\n\n**Imported package \`${subWord}\`**`;
    const exportedSymbols = Array.from(pkg.symbols.values()).map((s) => `\`${s.name}\``);
    if (exportedSymbols.length > 0) {
      md += `\n\n**Exported Symbols:**\n${exportedSymbols.join(', ')}`;
    }
    return {
      contents: {
        kind: MarkupKind.Markdown,
        value: md,
      },
    };
  }

  // 3. User-defined symbol in current document
  const sym = parsedDoc.symbols.get(subWord);
  if (sym) {
    let md = `\`\`\`go\n${sym.detail}\n\`\`\``;
    if (sym.kind === SymbolKind.Struct && sym.fields && sym.fields.length > 0) {
      md += '\n\n**Fields (Striped SoA Layout):**\n';
      for (const f of sym.fields) {
        md += `- \`${f.name}\`: \`${f.type}\`${f.docComment ? ' — ' + f.docComment : ''}\n`;
      }
    }
    if (sym.docComment) {
      md += `\n\n${sym.docComment}`;
    }
    return {
      contents: {
        kind: MarkupKind.Markdown,
        value: md,
      },
    };
  }

  // Check if hovering over a struct field name
  for (const struct of parsedDoc.structs.values()) {
    if (struct.fields) {
      const field = struct.fields.find((f) => f.name === subWord);
      if (field) {
        let md = `\`\`\`go\n(field) ${struct.name}.${field.name} ${field.type}\n\`\`\``;
        if (field.docComment) {
          md += `\n\n${field.docComment}`;
        }
        return {
          contents: {
            kind: MarkupKind.Markdown,
            value: md,
          },
        };
      }
    }
  }

  // 4. Built-in types
  if (M3_TYPES[subWord]) {
    const entry = M3_TYPES[subWord];
    const md = `\`\`\`go\n${entry.detail}\n\`\`\`\n\n${entry.documentation}`;
    return {
      contents: {
        kind: MarkupKind.Markdown,
        value: md,
      },
    };
  }

  // 5. Storage keywords
  if (M3_STORAGE[subWord]) {
    const entry = M3_STORAGE[subWord];
    const md = `\`\`\`go\n${entry.detail}\n\`\`\`\n\n${entry.documentation}`;
    return {
      contents: {
        kind: MarkupKind.Markdown,
        value: md,
      },
    };
  }

  // 6. Built-in intrinsics
  if (M3_INTRINSICS[subWord]) {
    const entry = M3_INTRINSICS[subWord];
    const md = `\`\`\`go\n${entry.detail}\n\`\`\`\n\n${entry.documentation}`;
    return {
      contents: {
        kind: MarkupKind.Markdown,
        value: md,
      },
    };
  }

  // 7. Language keywords
  if (M3_KEYWORDS[subWord]) {
    const entry = M3_KEYWORDS[subWord];
    const md = `\`\`\`go\n${entry.detail}\n\`\`\`\n\n${entry.documentation}`;
    return {
      contents: {
        kind: MarkupKind.Markdown,
        value: md,
      },
    };
  }

  return null;
}

export function getAsmHover(
  document: TextDocument,
  position: Position,
  parsedDoc: ParsedAsmDocument
): Hover | null {
  const wordCtx = getWordAtPosition(document, position);
  if (!wordCtx) return null;

  const rawWord = wordCtx.rawWord;

  // 1. User-defined symbol (labels, procs, macros, defines)
  const sym = parsedDoc.symbols.get(rawWord);
  if (sym) {
    let md = `\`\`\`assembly\n${sym.detail}\n\`\`\``;
    if (sym.kind === AsmSymbolKind.Procedure && sym.bank !== undefined) {
      md += `\n\n*Bank Context*: \`${sym.bank}\``;
    }
    if (sym.docComment) {
      md += `\n\n${sym.docComment}`;
    }
    return {
      contents: {
        kind: MarkupKind.Markdown,
        value: md,
      },
    };
  }

  // 2. 6502 Instructions (case-insensitive)
  const upperWord = rawWord.toUpperCase();
  if (ASM_INSTRUCTIONS[upperWord]) {
    const inst = ASM_INSTRUCTIONS[upperWord];
    let md = `### 6502 Instruction: \`${inst.name}\`\n\n${inst.detail}\n\n`;
    md += `**Flags Affected:** \`${inst.flags}\`\n\n`;
    md += `**Addressing Modes:**\n` + inst.modes.map((m) => `- ${m}`).join('\n') + '\n\n';
    md += inst.documentation;
    return {
      contents: {
        kind: MarkupKind.Markdown,
        value: md,
      },
    };
  }

  // 3. Assembler Directives (.directive or directive)
  const dirKey = rawWord.startsWith('.') ? rawWord.toLowerCase() : '.' + rawWord.toLowerCase();
  if (ASM_DIRECTIVES[dirKey]) {
    const dir = ASM_DIRECTIVES[dirKey];
    const md = `### Directive: \`${dir.name}\`\n\n\`\`\`assembly\n${dir.detail}\n\`\`\`\n\n${dir.documentation}`;
    return {
      contents: {
        kind: MarkupKind.Markdown,
        value: md,
      },
    };
  }

  // 4. NES Hardware Registers
  if (NES_REGISTERS[upperWord]) {
    const reg = NES_REGISTERS[upperWord];
    const md = `### NES Hardware Register: \`${reg.name}\`\n\n\`\`\`assembly\n${reg.detail}\n\`\`\`\n\n${reg.documentation}`;
    return {
      contents: {
        kind: MarkupKind.Markdown,
        value: md,
      },
    };
  }

  return null;
}
