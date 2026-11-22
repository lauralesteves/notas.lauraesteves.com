import { Home } from './components/Home/Home';
import { InvalidPath } from './components/InvalidPath/InvalidPath';
import { NotePage } from './components/NotePage/NotePage';
import type { ParsedLocation } from './lib/path';

export default function App({ route }: { route: ParsedLocation }) {
  switch (route.kind) {
    case 'home':
      return <Home />;
    case 'note':
      return <NotePage key={route.path} path={route.path} />;
    case 'invalid':
      return <InvalidPath path={route.path} error={route.error} />;
  }
}
