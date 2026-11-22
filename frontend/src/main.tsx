import { StrictMode } from 'react';
import { createRoot } from 'react-dom/client';
import App from './App';
import { parseLocation } from './lib/path';
import './index.css';

// No router: the pathname is the note. Canonicalize it before rendering.
const route = parseLocation(window.location.pathname);
const canonical =
  route.kind === 'invalid' ? `/${route.path}` : route.canonicalPathname;
if (canonical !== window.location.pathname) {
  window.history.replaceState(
    null,
    '',
    canonical + window.location.search + window.location.hash,
  );
}

createRoot(document.getElementById('root') as HTMLElement).render(
  <StrictMode>
    <App route={route} />
  </StrictMode>,
);
