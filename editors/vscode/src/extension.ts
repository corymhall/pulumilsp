// eslint-disable-next-line import/no-extraneous-dependencies
import * as vscode from 'vscode';
import {
  LanguageClient,
  LanguageClientOptions,
  ServerOptions,
} from 'vscode-languageclient/node';

let client: LanguageClient;

let OUTPUT_CHANNEL: vscode.OutputChannel | null = null;
export function outputChannel() {
  if (!OUTPUT_CHANNEL) {
    OUTPUT_CHANNEL = vscode.window.createOutputChannel('Pulumi LSP Server');
  }
  return OUTPUT_CHANNEL;
}

export async function activate(
  _context: vscode.ExtensionContext,
): Promise<LanguageClient> {
  const serverOptions: ServerOptions = {
    command: 'pulumilsp',
  };

  const clientOptions: LanguageClientOptions = {
    documentSelector: [{ scheme: 'file', language: 'typescript' }],
    progressOnInitialization: true,
  };

  // Create the language client and start the client.
  client = new LanguageClient(
    'pulumilsp',
    'Pulumi Diagnostics Language Server',
    serverOptions,
    clientOptions,
  );
  void client.start();

  return client;
}

export function deactivate(): Thenable<void> | undefined {
  if (!client) {
    return undefined;
  }

  return client.stop();
}
