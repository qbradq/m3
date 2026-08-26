const esbuild = require('esbuild');

const isWatch = process.argv.includes('--watch');

async function main() {
  const ctxClient = await esbuild.context({
    entryPoints: ['src/client/extension.ts'],
    bundle: true,
    format: 'cjs',
    minify: !isWatch,
    sourcemap: isWatch,
    sourcesContent: false,
    platform: 'node',
    outfile: 'dist/client/extension.js',
    external: ['vscode'],
    logLevel: 'info',
  });

  const ctxServer = await esbuild.context({
    entryPoints: ['src/server/server.ts'],
    bundle: true,
    format: 'cjs',
    minify: !isWatch,
    sourcemap: isWatch,
    sourcesContent: false,
    platform: 'node',
    outfile: 'dist/server/server.js',
    external: ['vscode'],
    logLevel: 'info',
  });

  if (isWatch) {
    await Promise.all([ctxClient.watch(), ctxServer.watch()]);
  } else {
    await Promise.all([ctxClient.rebuild(), ctxServer.rebuild()]);
    await Promise.all([ctxClient.dispose(), ctxServer.dispose()]);
  }
}

main().catch((e) => {
  console.error(e);
  process.exit(1);
});
