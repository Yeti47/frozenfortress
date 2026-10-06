// FrozenFortress local dev stack (Aspire TypeScript AppHost).
//
// Runs the webui natively with `go run` next to Redis and Ollama containers,
// so Go changes only need a restart from the dashboard instead of an image
// rebuild. This is for development only: production uses compose.yaml.
//
// Toggles (environment variables read when the AppHost starts):
//   FF_DEV_OCR=ollama     (default) Ollama container, provider ollama-tesseract
//   FF_DEV_OCR=tesseract  no Ollama; webui built with Tesseract (needs the
//                         Tesseract dev packages, see install-dev-deps-*.sh)
//   FF_DEV_OCR=nop        no OCR at all
//   FF_DEV_WEBUI=container  run the webui from the root Dockerfile instead of
//                           `go run`, to smoke-test the real image
//   FF_DEV_TLS=1          add nginx in front of the webui (https://localhost:18443)
//   FF_DEV_PORT           webui port on localhost (default 18080)
//   FF_DEV_HTTPS_PORT     nginx port on localhost (default 18443)
//
// The ports deliberately differ from the release stack (8443), so both can run
// side by side.
//
// See doc/dev-stack.md.

import { mkdirSync } from 'node:fs';
import path from 'node:path';
import { createBuilder, EndpointProperty } from './.aspire/modules/aspire.mjs';

const repoRoot = path.resolve(import.meta.dirname, '../..');
const dataDir = path.resolve(import.meta.dirname, '.data');

const ocrMode = (process.env.FF_DEV_OCR ?? 'ollama').toLowerCase();
const webuiInContainer = process.env.FF_DEV_WEBUI === 'container';
const withTls = process.env.FF_DEV_TLS === '1';
const webuiPort = Number(process.env.FF_DEV_PORT ?? 18080);
const httpsPort = Number(process.env.FF_DEV_HTTPS_PORT ?? 18443);

if (!['ollama', 'tesseract', 'nop'].includes(ocrMode)) {
    throw new Error(`FF_DEV_OCR must be ollama, tesseract or nop (got "${ocrMode}")`);
}
if (webuiInContainer && ocrMode === 'tesseract') {
    throw new Error('FF_DEV_OCR=tesseract needs the native webui; the image is built without Tesseract');
}

const ollamaModel = 'glm-ocr:q8_0';

const builder = await createBuilder();

// Same pinned image and settings as compose.yaml: sessions only, no persistence.
// redis:7.4.2-alpine
const redis = await builder
    .addContainer('redis', 'redis')
    .withImageSHA256('02419de7eddf55aa5bcf49efb74e88fa8d931b4d77c07eff8a6b2144472b6952')
    .withContainerRuntimeArgs(['--user', '999:1000'])
    .withArgs(['redis-server', '--save', '', '--appendonly', 'no'])
    .withEndpoint({ name: 'tcp', targetPort: 6379 });

// CPU variant of the release Ollama image. The model lives in a named volume
// so it is only downloaded once.
const ollama = ocrMode === 'ollama'
    ? await builder
        .addDockerfile('ollama', path.join(repoRoot, 'docker/ollama'), { stage: 'cpu' })
        .withEnvironment('OLLAMA_MODEL', ollamaModel)
        .withVolume('/models', { name: 'frozenfortress-dev-ollama' })
        .withHttpEndpoint({ targetPort: 11434 })
    : undefined;

const ocrProvider = { ollama: 'ollama-tesseract', tesseract: 'tesseract', nop: 'nop' }[ocrMode]!;

let webuiHttp;
if (webuiInContainer) {
    const webui = await builder
        .addDockerfile('webui', repoRoot)
        .withBuildArg('APP_VERSION', 'dev')
        .withVolume('/data', { name: 'frozenfortress-dev-data' })
        .withEnvironment('FF_REDIS_ADDRESS', redis.getEndpoint('tcp').property(EndpointProperty.HostAndPort))
        .withEnvironment('FF_OCR_PROVIDER', ocrProvider)
        .withEnvironment('FF_OCR_OLLAMA_MODEL', ollamaModel)
        .withEnvironment('FF_LOG_LEVEL', 'Debug')
        .withEnvironment('FF_UPDATE_CHECK_ENABLED', 'false')
        .withHttpEndpoint({ port: webuiPort, targetPort: 8080, env: 'FF_WEB_UI_PORT' })
        .waitFor(redis);
    if (ollama) {
        await webui.withEnvironment('FF_OCR_OLLAMA_URL', ollama.getEndpoint('http'));
    }
    webuiHttp = webui.getEndpoint('http');
} else {
    mkdirSync(dataDir, { recursive: true });

    // Runs from webui/ because templates and static files are loaded relative
    // to the working directory.
    const webui = await builder
        .addGoApp('webui', path.join(repoRoot, 'webui'), {
            buildTags: ocrMode === 'tesseract' ? [] : ['notesseract'],
            ldFlags: '-X github.com/Yeti47/frozenfortress/frozenfortress/core/ccc.AppVersion=dev',
        })
        .withEnvironment('FF_DATABASE_PATH', path.join(dataDir, 'frozenfortress.db'))
        .withEnvironment('FF_KEY_DIR', path.join(dataDir, 'keys'))
        .withEnvironment('FF_BACKUP_DIRECTORY', path.join(dataDir, 'backups'))
        .withEnvironment('FF_REDIS_ADDRESS', redis.getEndpoint('tcp').property(EndpointProperty.HostAndPort))
        .withEnvironment('FF_OCR_PROVIDER', ocrProvider)
        .withEnvironment('FF_OCR_OLLAMA_MODEL', ollamaModel)
        .withEnvironment('FF_LOG_LEVEL', 'Debug')
        .withEnvironment('FF_UPDATE_CHECK_ENABLED', 'false')
        .withHttpEndpoint({ port: webuiPort, env: 'FF_WEB_UI_PORT' })
        .waitFor(redis);
    // OCR is async and best-effort, so the webui does not wait for Ollama.
    if (ollama) {
        await webui.withEnvironment('FF_OCR_OLLAMA_URL', ollama.getEndpoint('http'));
    }
    webuiHttp = webui.getEndpoint('http');
}

if (withTls) {
    // The release nginx image proxies to webui:8080 on the compose network.
    // Here the webui is elsewhere, so rewrite the upstream into a copy of the
    // config at start. The image's entrypoint still bootstraps the certificate.
    await builder
        .addDockerfile('nginx', path.join(repoRoot, 'docker/nginx'))
        .withEnvironment('FF_TLS_CERT_PATH', '/data/certs/frozenfortress.crt')
        .withEnvironment('FF_TLS_KEY_PATH', '/data/certs/frozenfortress.key')
        .withEnvironment('FF_TLS_COMMON_NAME', 'frozenfortress.local')
        .withEnvironment('FF_TLS_HOSTS', 'localhost,frozenfortress.local')
        .withEnvironment('FF_UPSTREAM', webuiHttp.property(EndpointProperty.HostAndPort))
        .withVolume('/data/certs', { name: 'frozenfortress-dev-certs' })
        .withArgs([
            'sh', '-c',
            'sed "s|server webui:8080;|server $FF_UPSTREAM;|" /etc/nginx/nginx.conf > /tmp/nginx/nginx.conf'
            + ' && exec nginx -c /tmp/nginx/nginx.conf -g "daemon off;"',
        ])
        .withEndpoint({ name: 'https', scheme: 'https', port: httpsPort, targetPort: 8443 });
}

await builder.build().run();
