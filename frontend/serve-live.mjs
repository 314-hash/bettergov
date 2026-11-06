import http from 'node:http';
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const PORT = Number(process.env.FRONTEND_PORT || 5173);

const server = http.createServer((req, res) => {
  if (req.url === '/logo.png') {
    const logoPath = path.join(__dirname, 'public/logo.png');
    if (fs.existsSync(logoPath)) {
      res.writeHead(200, { 'Content-Type': 'image/png' });
      fs.createReadStream(logoPath).pipe(res);
      return;
    }
  }

  // Read App.tsx and rpcClient.ts and transform TypeScript types for in-browser Babel execution
  const rpcClientSrc = fs
    .readFileSync(path.join(__dirname, 'src/services/rpcClient.ts'), 'utf8')
    .replace(/import\s+type\s+\{[\s\S]*?\}\s+from\s+['"][^'"]+['"];?/g, '')
    .replace(/export\s+const\s+/g, 'const ')
    .replace(/export\s+async\s+function\s+/g, 'async function ');

  const appTsxSrc = fs
    .readFileSync(path.join(__dirname, 'src/App.tsx'), 'utf8')
    .replace(/import\s+[\s\S]*?from\s+['"][^'"]+['"];?/g, '')
    .replace(/export\s+function\s+App/g, 'function App');

  const html = `<!DOCTYPE html>
<html lang="en" class="dark">
  <head>
    <meta charset="UTF-8" />
    <meta name="viewport" content="width=device-width, initial-scale=1.0" />
    <title>BETTERGOV Chain (BGOV) — Sovereign Layer 1 Dashboard</title>
    <link rel="icon" type="image/png" href="/logo.png" />
    <script src="https://cdn.tailwindcss.com"></script>
    <script src="https://unpkg.com/react@18/umd/react.production.min.js" crossorigin></script>
    <script src="https://unpkg.com/react-dom@18/umd/react-dom.production.min.js" crossorigin></script>
    <script src="https://unpkg.com/@babel/standalone/babel.min.js"></script>
  </head>
  <body class="bg-[#0A1628] text-slate-100 antialiased selection:bg-[#D4AF37] selection:text-[#0A1628]">
    <div id="root"></div>
    <script type="text/babel" data-presets="typescript,react">
      const { useState, useEffect } = React;
      ${rpcClientSrc}
      ${appTsxSrc}
      ReactDOM.createRoot(document.getElementById('root')).render(<App />);
    </script>
  </body>
</html>`;

  res.writeHead(200, { 'Content-Type': 'text/html; charset=utf-8' });
  res.end(html);
});

server.listen(PORT, '127.0.0.1', () => {
  console.log(`[BETTERGOV Frontend] Live 11-Page Web App running at http://127.0.0.1:${PORT}`);
});
