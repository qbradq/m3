import * as path from 'path';
import { ExtensionContext, workspace } from 'vscode';
import {
  LanguageClient,
  LanguageClientOptions,
  ServerOptions,
  TransportKind,
} from 'vscode-languageclient/node';

let client: LanguageClient;

export function activate(context: ExtensionContext) {
  // Path to the language server module
  const serverModule = context.asAbsolutePath(path.join('dist', 'server', 'server.js'));

  // Server options for run and debug modes
  const serverOptions: ServerOptions = {
    run: { module: serverModule, transport: TransportKind.ipc },
    debug: {
      module: serverModule,
      transport: TransportKind.ipc,
      options: { execArgv: ['--nolazy', '--inspect=6009'] },
    },
  };

  // Client options controlling document selectors and synchronization
  const clientOptions: LanguageClientOptions = {
    documentSelector: [
      { scheme: 'file', language: 'm3' },
      { scheme: 'file', language: 'm3-asm' },
    ],
    synchronize: {
      fileEvents: workspace.createFileSystemWatcher('**/*.{m3,s,asm,inc}'),
    },
  };

  // Create and start the language client
  client = new LanguageClient(
    'm3LanguageServer',
    'm3 Language Server',
    serverOptions,
    clientOptions
  );

  client.start();
}

export function deactivate(): Thenable<void> | undefined {
  if (!client) {
    return undefined;
  }
  return client.stop();
}
