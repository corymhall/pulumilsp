import * as fs from 'fs';
import * as process from 'process';

// eslint-disable-next-line import/no-extraneous-dependencies
import * as vscode from 'vscode';
import {
  LanguageClient,
  LanguageClientOptions,
  ServerOptions,
} from 'vscode-languageclient/node';

let client: LanguageClient;

class Config {
  readonly rootPath: string = 'pulumilsp';
  constructor() {
    vscode.workspace.onDidChangeConfiguration(this.onDidChangeConfiguration);
  }

  serverPath() {
    return vscode.workspace
      .getConfiguration(this.rootPath)
      .get<string | null>('server.path');
  }

  onDidChangeConfiguration(event: vscode.ConfigurationChangeEvent) {
    if (event.affectsConfiguration(this.rootPath)) {
      outputChannel().replace(
        'Restart the Pulumi LSP extension for configuration changes to take effect.',
      );
    }
  }
}

let OUTPUT_CHANNEL: vscode.OutputChannel | null = null;
export function outputChannel() {
  if (!OUTPUT_CHANNEL) {
    OUTPUT_CHANNEL = vscode.window.createOutputChannel('Pulumi LSP Server');
  }
  return OUTPUT_CHANNEL;
}

async function getServer(
  context: vscode.ExtensionContext,
  config: Config,
): Promise<string | undefined> {
  const explicitPath = config.serverPath();
  if (explicitPath) {
    if (fs.existsSync(explicitPath)) {
      outputChannel().replace(
        `Launching server from explicitly provided path: ${explicitPath}`,
      );
      return Promise.resolve(explicitPath);
    }
    const msg = `${config.rootPath}.server.path specified a path, but the file ${explicitPath} does not exist.`;
    outputChannel().replace(msg);
    outputChannel().show();
    return Promise.reject(msg);
  }

  const ext = process.platform === 'win32' ? '.exe' : '';
  const bundled = vscode.Uri.joinPath(context.extensionUri, `pulumilsp${ext}`);
  const bundledExists = await vscode.workspace.fs.stat(bundled).then(
    () => true,
    () => false,
  );

  if (bundledExists) {
    const path = bundled.fsPath;
    outputChannel().replace(`Launching built-in Pulumi Diagnostics LSP Server`);
    return Promise.resolve(path);
  }

  outputChannel().replace(`Could not find a bundled Pulumi LSP Server.
Please specify a pulumilsp binary via settings.json at the "${config.rootPath}.server.path" key.
If you think this is an error, please report it at https://github.com/corymhall/pulumilsp/issues.`);
  outputChannel().show();

  return Promise.reject('No binary found');
}

export async function activate(
  context: vscode.ExtensionContext,
): Promise<LanguageClient> {
  const config = new Config();
  const serverPath = await getServer(context, config);
  if (serverPath === undefined) {
    outputChannel().append('\nFailed to find LSP executable');
    return Promise.reject();
  }
  const serverOptions: ServerOptions = {
    command: serverPath,
  };

  const clientOptions: LanguageClientOptions = {
    documentSelector: [{ scheme: 'file', language: 'typescript' }],
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
