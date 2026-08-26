import {
  createConnection,
  TextDocuments,
  ProposedFeatures,
  InitializeParams,
  InitializeResult,
  TextDocumentSyncKind,
  CompletionItem,
  TextDocumentPositionParams,
  Hover,
} from 'vscode-languageserver/node';
import { TextDocument } from 'vscode-languageserver-textdocument';
import { parseM3Document, ParsedM3Document } from './parser/m3Parser';
import { parseAsmDocument, ParsedAsmDocument } from './parser/asmParser';
import { getM3Completions, getAsmCompletions } from './providers/completionProvider';
import { getM3Hover, getAsmHover } from './providers/hoverProvider';

// Create connection for the server using IPC
const connection = createConnection(ProposedFeatures.all);

// Create a text document manager
const documents: TextDocuments<TextDocument> = new TextDocuments(TextDocument);

// Parsed document symbol caches
const m3DocCache = new Map<string, ParsedM3Document>();
const asmDocCache = new Map<string, ParsedAsmDocument>();

function isAsmDocument(document: TextDocument): boolean {
  if (document.languageId === 'm3-asm') return true;
  const uri = document.uri.toLowerCase();
  return uri.endsWith('.s') || uri.endsWith('.asm') || uri.endsWith('.inc');
}

function updateDocumentCache(document: TextDocument) {
  const text = document.getText();
  if (isAsmDocument(document)) {
    asmDocCache.set(document.uri, parseAsmDocument(text));
    m3DocCache.delete(document.uri);
  } else {
    m3DocCache.set(document.uri, parseM3Document(text, document.uri));
    asmDocCache.delete(document.uri);
  }
}

connection.onInitialize((_params: InitializeParams): InitializeResult => {
  return {
    capabilities: {
      textDocumentSync: TextDocumentSyncKind.Incremental,
      completionProvider: {
        resolveProvider: false,
        triggerCharacters: ['.', '@', ':', '"'],
      },
      hoverProvider: true,
    },
  };
});

connection.onInitialized(() => {
  connection.console.log('m3 Language Server initialized successfully with Symbol Completion & Documentation.');
});

documents.onDidChangeContent((change) => {
  updateDocumentCache(change.document);
});

documents.onDidClose((e) => {
  m3DocCache.delete(e.document.uri);
  asmDocCache.delete(e.document.uri);
});

connection.onCompletion((params: TextDocumentPositionParams): CompletionItem[] => {
  const document = documents.get(params.textDocument.uri);
  if (!document) return [];

  if (isAsmDocument(document)) {
    let parsedDoc = asmDocCache.get(document.uri);
    if (!parsedDoc) {
      parsedDoc = parseAsmDocument(document.getText());
      asmDocCache.set(document.uri, parsedDoc);
    }
    return getAsmCompletions(document, params.position, parsedDoc);
  } else {
    let parsedDoc = m3DocCache.get(document.uri);
    if (!parsedDoc) {
      parsedDoc = parseM3Document(document.getText(), document.uri);
      m3DocCache.set(document.uri, parsedDoc);
    }
    return getM3Completions(document, params.position, parsedDoc);
  }
});

connection.onHover((params: TextDocumentPositionParams): Hover | null => {
  const document = documents.get(params.textDocument.uri);
  if (!document) return null;

  if (isAsmDocument(document)) {
    let parsedDoc = asmDocCache.get(document.uri);
    if (!parsedDoc) {
      parsedDoc = parseAsmDocument(document.getText());
      asmDocCache.set(document.uri, parsedDoc);
    }
    return getAsmHover(document, params.position, parsedDoc);
  } else {
    let parsedDoc = m3DocCache.get(document.uri);
    if (!parsedDoc) {
      parsedDoc = parseM3Document(document.getText(), document.uri);
      m3DocCache.set(document.uri, parsedDoc);
    }
    return getM3Hover(document, params.position, parsedDoc);
  }
});

// Listen on the documents manager and connection
documents.listen(connection);
connection.listen();
