import { Hover, MarkupKind, Position } from 'vscode-languageserver/node';
import { TextDocument } from 'vscode-languageserver-textdocument';
import { M3_TYPES, M3_STORAGE, M3_INTRINSICS, M3_KEYWORDS } from '../data/m3Docs';
import { ASM_INSTRUCTIONS, ASM_DIRECTIVES, NES_REGISTERS } from '../data/asmDocs';
import { ParsedM3Document, SymbolKind } from '../parser/m3Parser';
import { ParsedAsmDocument, AsmSymbolKind } from '../parser/asmParser';

/**
 * Extracts the full word/identifier under the cursor, including dots (for directives) or @ (for local labels).
 */
export function getWordAtPosition(document: TextDocument, position: Position): string | null {
  const text = document.getText({
    start: { line: position.line, character: 0 },
    end: { line: position.line + 1, character: 0 },
  });

  const line = text.replace(/\r?\n$/, '');
  const charIdx = position.character;

  if (charIdx < 0 || charIdx > line.length) {
    return null;
  }

  // Regex to match identifiers, directives (with leading .), or local labels (with leading @)
  const regex = /([.@a-zA-Z_][a-zA-Z0-9_:]*)/g;
  let match: RegExpExecArray | null;

  while ((match = regex.exec(line)) !== null) {
    const start = match.index;
    const end = start + match[0].length;
    if (charIdx >= start && charIdx <= end) {
      return match[0];
    }
  }

  return null;
}

export function getM3Hover(
  document: TextDocument,
  position: Position,
  parsedDoc: ParsedM3Document
): Hover | null {
  const word = getWordAtPosition(document, position);
  if (!word) return null;

  // 1. User-defined symbol in current document
  const sym = parsedDoc.symbols.get(word);
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
      const field = struct.fields.find((f) => f.name === word);
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

  // 2. Built-in types
  if (M3_TYPES[word]) {
    const entry = M3_TYPES[word];
    const md = `\`\`\`go\n${entry.detail}\n\`\`\`\n\n${entry.documentation}`;
    return {
      contents: {
        kind: MarkupKind.Markdown,
        value: md,
      },
    };
  }

  // 3. Storage keywords
  if (M3_STORAGE[word]) {
    const entry = M3_STORAGE[word];
    const md = `\`\`\`go\n${entry.detail}\n\`\`\`\n\n${entry.documentation}`;
    return {
      contents: {
        kind: MarkupKind.Markdown,
        value: md,
      },
    };
  }

  // 4. Built-in intrinsics
  if (M3_INTRINSICS[word]) {
    const entry = M3_INTRINSICS[word];
    const md = `\`\`\`go\n${entry.detail}\n\`\`\`\n\n${entry.documentation}`;
    return {
      contents: {
        kind: MarkupKind.Markdown,
        value: md,
      },
    };
  }

  // 5. Language keywords
  if (M3_KEYWORDS[word]) {
    const entry = M3_KEYWORDS[word];
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
  const rawWord = getWordAtPosition(document, position);
  if (!rawWord) return null;

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
